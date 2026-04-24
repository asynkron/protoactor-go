package natsstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	natsstreammetrics "github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream/metrics"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/metric"
)

// Compile-time interface checks.
var _ cluster.ClusterProvider = (*Provider)(nil)
var _ cluster.SingletonSchedulerRegistrar = (*Provider)(nil)

// Provider uses NATS JetStream raw streams for cluster membership discovery,
// health checking via local timeout-based crash detection, and leader election
// via publish-race CAS semantics.
type Provider struct {
	cluster      *cluster.Cluster
	clusterName  string
	prefix       string // resolved subject prefix
	config       *config
	js           jetstream.JetStream
	stream       jetstream.Stream // cluster membership stream
	self         *Node
	isMember     bool
	members      map[string]*Node
	membersMu    sync.RWMutex
	lastSeen     map[string]time.Time // memberID -> last heartbeat time
	lastSeenMu   sync.RWMutex
	shutdown     atomic.Bool
	clusterError error
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc

	// Leader election
	role                cluster.RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan cluster.RoleType
	roleChangedListener cluster.RoleChangedListener
	schedulers          []cluster.RoleChangedListener
	isLeader            atomic.Bool
	leaderSeq           uint64 // last successful leader publish sequence
	leaderMemberID      string // ID of the current leader (from consumed messages)
	leaderMu            sync.RWMutex

	// Integrated identity lookup
	identity *IdentityLookup

	// Metrics
	providerMetrics *natsstreammetrics.NatsStreamMetrics
	metricsEnabled  bool
}

// New creates a Provider using a NATS connection. It creates a JetStream
// context from the connection and delegates to NewFromJetStream.
func New(conn *nats.Conn, opts ...Option) (*Provider, error) {
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("natsstream: create JetStream context: %w", err)
	}
	return NewFromJetStream(js, opts...)
}

// NewFromJetStream creates a Provider using an existing JetStream handle.
func NewFromJetStream(js jetstream.JetStream, opts ...Option) (*Provider, error) {
	cfg := newDefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	p := &Provider{
		config:              cfg,
		js:                  js,
		members:             make(map[string]*Node),
		lastSeen:            make(map[string]time.Time),
		role:                cluster.RoleFollower,
		roleChangedChan:     make(chan cluster.RoleType, 1),
		roleChangedListener: cfg.RoleChanged,
	}

	p.identity = newIdentityLookup(p)

	return p, nil
}

// IdentityLookup returns the integrated identity lookup.
func (p *Provider) IdentityLookup() *IdentityLookup {
	return p.identity
}

// GetHealthStatus returns an error if the cluster health status has problems.
func (p *Provider) GetHealthStatus() error {
	return p.clusterError
}

// RegisterSingletonScheduler registers a RoleChangedListener to be notified on
// leadership role changes. Safe to call before or after StartMember.
// If the node is already leader, the listener is immediately notified.
func (p *Provider) RegisterSingletonScheduler(listener cluster.RoleChangedListener) {
	if p.shutdown.Load() {
		return
	}
	p.roleMu.Lock()
	defer p.roleMu.Unlock()
	p.schedulers = append(p.schedulers, listener)
	if p.role == cluster.RoleLeader {
		cluster.SafeRunRoleChange(p.logger(), func() {
			listener.OnRoleChanged(cluster.RoleLeader)
		})
	}
}

// init extracts host, port, memberID, and kinds from the cluster and builds the self node.
func (p *Provider) init(c *cluster.Cluster) error {
	p.cluster = c
	p.clusterName = c.Config.Name
	p.prefix = p.config.subjectPrefix(p.clusterName)

	addr := c.ActorSystem.Address()
	host, port, err := splitHostPort(addr)
	if err != nil {
		return err
	}

	memberID := c.ActorSystem.ID
	knownKinds := c.GetClusterKinds()
	nodeName := fmt.Sprintf("%v_%v", p.clusterName, memberID)
	p.self = NewNode(nodeName, host, port, knownKinds)
	return nil
}

// StartMember registers the node and starts all background goroutines.
func (p *Provider) StartMember(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.isMember = true

	if err := p.init(c); err != nil {
		return err
	}

	p.providerMetrics = natsstreammetrics.NewNatsStreamMetrics(c.Logger())
	p.metricsEnabled = c.MetricsEnabled()

	if err := p.createClusterStream(); err != nil {
		return err
	}

	if err := p.publishJoin(); err != nil {
		return err
	}

	if err := p.publishHeartbeat(); err != nil {
		return err
	}

	p.startRoleChangedNotifyLoop()
	p.startStreamConsumer()
	p.startHeartbeatPublisher()
	p.startStaleMemberChecker()
	p.startLeaderElection()

	return nil
}

// StartClient initializes the provider without registering the node.
func (p *Provider) StartClient(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())

	if err := p.init(c); err != nil {
		return err
	}

	p.providerMetrics = natsstreammetrics.NewNatsStreamMetrics(c.Logger())
	p.metricsEnabled = c.MetricsEnabled()

	if err := p.createClusterStream(); err != nil {
		return err
	}

	p.startStreamConsumer()

	return nil
}

// Shutdown deregisters the node and stops background tasks.
func (p *Provider) Shutdown(_ bool) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil
	}

	// Publish leave event before cancelling context
	if p.isMember && p.self != nil {
		leaveCtx, leaveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer leaveCancel()
		p.publishLeave(leaveCtx)
	}

	if p.cancel != nil {
		p.cancel()
	}

	// Wait for all goroutines to finish
	p.wg.Wait()

	return nil
}

// UpdateKinds updates the node's kind list and publishes an immediate
// heartbeat so other nodes see the change without waiting for the next
// heartbeat cycle.
func (p *Provider) UpdateKinds(kinds []string) error {
	p.membersMu.Lock()
	p.self.Kinds = kinds
	p.membersMu.Unlock()

	return p.publishHeartbeat()
}

// createClusterStream creates or binds the cluster membership stream.
func (p *Provider) createClusterStream() error {
	name := p.config.streamName(p.clusterName)

	s, err := p.js.CreateOrUpdateStream(p.ctx, jetstream.StreamConfig{
		Name:                   name,
		Subjects:               []string{p.prefix + ".>"},
		MaxMsgsPerSubject:      1,
		MaxAge:                 p.config.MaxAge,
		AllowMsgTTL:            true,
		SubjectDeleteMarkerTTL: p.config.HeartbeatTTL,
		Retention:              jetstream.LimitsPolicy,
		Storage:                p.config.Storage,
		Replicas:               p.config.Replicas,
	})
	if err != nil {
		return fmt.Errorf("natsstream: create cluster stream %q: %w", name, err)
	}

	p.stream = s
	return nil
}

// publishJoin publishes a join event to the stream.
func (p *Provider) publishJoin() error {
	type joinPayload struct {
		ID    string   `json:"id"`
		Host  string   `json:"host"`
		Port  int      `json:"port"`
		Kinds []string `json:"kinds"`
	}
	payload := joinPayload{
		ID:    p.self.ID,
		Host:  p.self.Host,
		Port:  p.self.Port,
		Kinds: p.self.Kinds,
	}
	data, err := marshalJSON(payload)
	if err != nil {
		return fmt.Errorf("natsstream: marshal join: %w", err)
	}

	subject := p.prefix + ".join." + p.self.ID
	_, err = p.js.Publish(p.ctx, subject, data)
	if err != nil {
		return fmt.Errorf("natsstream: publish join: %w", err)
	}
	return nil
}

// publishHeartbeat publishes a heartbeat message with per-message TTL.
func (p *Provider) publishHeartbeat() error {
	p.membersMu.RLock()
	data, err := p.self.Serialize()
	p.membersMu.RUnlock()
	if err != nil {
		return fmt.Errorf("natsstream: serialize self: %w", err)
	}

	subject := p.prefix + ".members." + p.self.ID
	_, err = p.js.Publish(p.ctx, subject, data, jetstream.WithMsgTTL(p.config.HeartbeatTTL))
	if err != nil {
		return fmt.Errorf("natsstream: publish heartbeat: %w", err)
	}
	return nil
}

// publishLeave publishes a leave event.
func (p *Provider) publishLeave(ctx context.Context) {
	type leavePayload struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	payload := leavePayload{ID: p.self.ID, Reason: "shutdown"}
	data, _ := marshalJSON(payload)

	subject := p.prefix + ".leave." + p.self.ID
	_, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		p.logger().Warn("Failed to publish leave event",
			slog.String("provider", "natsstream"),
			slog.Any("error", err))
	}
}

// startHeartbeatPublisher starts the goroutine that periodically publishes heartbeats.
func (p *Provider) startHeartbeatPublisher() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in heartbeat publisher",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
			}
		}()

		ticker := time.NewTicker(p.config.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				if err := p.publishHeartbeat(); err != nil {
					p.logger().Warn("Failed to publish heartbeat",
						slog.String("provider", "natsstream"),
						slog.Any("error", err))
					if p.metricsEnabled {
						p.providerMetrics.KeyRefreshFailureCount.Add(
							context.Background(), 1,
							metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
						)
					}
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// startStreamConsumer starts the ordered consumer goroutine that processes all cluster events.
func (p *Provider) startStreamConsumer() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in stream consumer",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
				p.clusterError = fmt.Errorf("stream consumer panic: %v", r)
			}
		}()

		for !p.shutdown.Load() {
			if err := p.consumeStream(); err != nil {
				if p.shutdown.Load() {
					return
				}
				p.logger().Error("Stream consumer failed, retrying",
					slog.String("provider", "natsstream"),
					slog.Any("error", err))
				p.clusterError = err
				if p.metricsEnabled {
					p.providerMetrics.WatchReconnectCount.Add(
						context.Background(), 1,
						metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
					)
				}

				select {
				case <-time.After(p.config.RetryInterval):
				case <-p.ctx.Done():
					return
				}
			}
		}
	}()
}

// consumeStream creates an ordered consumer and processes messages.
func (p *Provider) consumeStream() error {
	streamName := p.config.streamName(p.clusterName)
	consumer, err := p.js.OrderedConsumer(p.ctx, streamName, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{p.prefix + ".>"},
	})
	if err != nil {
		return fmt.Errorf("natsstream: create ordered consumer: %w", err)
	}

	iter, err := consumer.Messages()
	if err != nil {
		return fmt.Errorf("natsstream: get message iterator: %w", err)
	}
	defer iter.Stop()

	// Stop the iterator when the context is cancelled so that iter.Next()
	// unblocks. Without this, iter.Next() can block indefinitely since it
	// does not natively check a context.
	go func() {
		<-p.ctx.Done()
		iter.Stop()
	}()

	// Track whether we've done the initial replay.
	// After processing all existing messages, publish initial topology.
	initialPublished := false

	for {
		msg, err := iter.Next()
		if err != nil {
			if p.ctx.Err() != nil {
				return nil // context cancelled, graceful exit
			}
			return fmt.Errorf("natsstream: next message: %w", err)
		}

		p.handleStreamMessage(msg)
		msg.Ack()

		// After processing each message, check if there are more pending.
		// If not, and we haven't published initial topology yet, do so now.
		if !initialPublished {
			info, infoErr := consumer.Info(p.ctx)
			if infoErr == nil && info.NumPending == 0 {
				initialPublished = true
				p.publishClusterTopologyEvent()
			}
		}
	}
}

// handleStreamMessage routes a stream message to the appropriate handler based on subject.
func (p *Provider) handleStreamMessage(msg jetstream.Msg) {
	subject := msg.Subject()

	switch {
	case subjectMatches(subject, p.prefix+".members."):
		p.handleHeartbeatMessage(msg)
	case subjectMatches(subject, p.prefix+".join."):
		// Join events are informational; the heartbeat carries the actual member data.
		// Log for observability.
		p.logger().Info("Member join event received",
			slog.String("provider", "natsstream"),
			slog.String("subject", subject))
	case subjectMatches(subject, p.prefix+".leave."):
		p.handleLeaveMessage(msg)
	case subject == p.prefix+".leader":
		p.handleLeaderMessage(msg)
	}
}

// handleHeartbeatMessage processes a heartbeat message from a member.
func (p *Provider) handleHeartbeatMessage(msg jetstream.Msg) {
	// Skip empty messages (e.g. delete markers from TTL expiry).
	if len(msg.Data()) == 0 {
		return
	}

	node, err := NewNodeFromBytes(msg.Data())
	if err != nil {
		p.logger().Error("Invalid heartbeat data",
			slog.String("provider", "natsstream"),
			slog.String("subject", msg.Subject()),
			slog.Any("error", err))
		return
	}

	// Don't track self via the consumer.
	if p.self != nil && node.Equal(p.self) {
		return
	}

	// Check if this is a stale replay message.
	meta, metaErr := msg.Metadata()
	if metaErr == nil {
		age := time.Since(meta.Timestamp)
		if age > p.config.MemberTimeout {
			// Stale message from replay -- ignore.
			return
		}
	}

	p.membersMu.Lock()
	p.members[node.ID] = node
	p.membersMu.Unlock()

	// Update last seen with local time (not message timestamp) to avoid clock skew.
	p.lastSeenMu.Lock()
	p.lastSeen[node.ID] = time.Now()
	p.lastSeenMu.Unlock()

	p.publishClusterTopologyEvent()
}

// handleLeaveMessage processes a leave event from a member.
func (p *Provider) handleLeaveMessage(msg jetstream.Msg) {
	type leavePayload struct {
		ID string `json:"id"`
	}
	var payload leavePayload
	if err := unmarshalJSON(msg.Data(), &payload); err != nil {
		return
	}

	// Skip self
	if p.self != nil && payload.ID == p.self.ID {
		return
	}

	p.membersMu.Lock()
	delete(p.members, payload.ID)
	p.membersMu.Unlock()

	p.lastSeenMu.Lock()
	delete(p.lastSeen, payload.ID)
	p.lastSeenMu.Unlock()

	p.logger().Info("Member left",
		slog.String("provider", "natsstream"),
		slog.String("memberID", payload.ID))

	p.publishClusterTopologyEvent()
}

// handleLeaderMessage processes a leader claim message.
func (p *Provider) handleLeaderMessage(msg jetstream.Msg) {
	type leaderPayload struct {
		MemberID string `json:"memberID"`
	}
	var payload leaderPayload
	if err := unmarshalJSON(msg.Data(), &payload); err != nil {
		return
	}

	p.leaderMu.Lock()
	p.leaderMemberID = payload.MemberID
	p.leaderMu.Unlock()
}

// startStaleMemberChecker starts the goroutine that periodically checks for
// members whose heartbeats have timed out.
func (p *Provider) startStaleMemberChecker() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in stale member checker",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
			}
		}()

		ticker := time.NewTicker(p.config.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				p.checkStaleMembersOnce()
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// checkStaleMembersOnce scans the lastSeen map and removes members that have
// exceeded the member timeout.
func (p *Provider) checkStaleMembersOnce() {
	now := time.Now()
	var stale []string

	p.lastSeenMu.RLock()
	for id, last := range p.lastSeen {
		if now.Sub(last) > p.config.MemberTimeout {
			stale = append(stale, id)
		}
	}
	p.lastSeenMu.RUnlock()

	if len(stale) == 0 {
		return
	}

	p.membersMu.Lock()
	for _, id := range stale {
		delete(p.members, id)
	}
	p.membersMu.Unlock()

	p.lastSeenMu.Lock()
	for _, id := range stale {
		delete(p.lastSeen, id)
	}
	p.lastSeenMu.Unlock()

	for _, id := range stale {
		p.logger().Warn("Stale member removed",
			slog.String("provider", "natsstream"),
			slog.String("memberID", id))
	}

	p.publishClusterTopologyEvent()
}

// startLeaderElection starts the goroutine that periodically attempts leader election.
func (p *Provider) startLeaderElection() {
	// Attempt immediately
	p.attemptLeaderElection()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in leader election",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
			}
		}()

		// Attempt interval: slightly shorter than leader TTL
		interval := p.config.LeaderTTL * 7 / 10
		if interval < 1*time.Second {
			interval = 1 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				p.attemptLeaderElection()
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// attemptLeaderElection tries to claim or refresh leadership via CAS publish.
func (p *Provider) attemptLeaderElection() {
	subject := p.prefix + ".leader"

	type leaderPayload struct {
		MemberID  string `json:"memberID"`
		ElectedAt string `json:"electedAt"`
	}
	payload := leaderPayload{
		MemberID:  p.self.ID,
		ElectedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := marshalJSON(payload)

	if p.isLeader.Load() {
		// Leader refresh: use the last known sequence
		seq := atomic.LoadUint64(&p.leaderSeq)
		ack, err := p.js.Publish(p.ctx, subject, data,
			jetstream.WithExpectLastSequencePerSubject(seq),
			jetstream.WithMsgTTL(p.config.LeaderTTL))
		if err != nil {
			// Lost leadership
			p.isLeader.Store(false)
			atomic.StoreUint64(&p.leaderSeq, 0)
			p.setRole(cluster.RoleFollower)
			return
		}
		atomic.StoreUint64(&p.leaderSeq, ack.Sequence)
	} else {
		// Non-leader attempt: try with seq=0 (expects no prior message)
		ack, err := p.js.Publish(p.ctx, subject, data,
			jetstream.WithExpectLastSequencePerSubject(0),
			jetstream.WithMsgTTL(p.config.LeaderTTL))
		if err != nil {
			// Another member is leader -- this is expected
			return
		}
		// Won leadership
		p.isLeader.Store(true)
		atomic.StoreUint64(&p.leaderSeq, ack.Sequence)
		p.setRole(cluster.RoleLeader)
		if p.metricsEnabled {
			p.providerMetrics.LeaderElectionCount.Add(
				context.Background(), 1,
				metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
			)
		}
	}
}

// publishClusterTopologyEvent converts internal state to cluster.Member list
// and notifies the cluster's MemberList.
func (p *Provider) publishClusterTopologyEvent() {
	start := time.Now()

	p.membersMu.RLock()
	members := make([]*cluster.Member, 0, len(p.members)+1)
	for _, m := range p.members {
		if m.IsAlive() {
			members = append(members, m.MemberStatus())
		}
	}
	p.membersMu.RUnlock()

	// Include self if registered as a member (not client).
	if p.self != nil && p.isMember {
		members = append(members, p.self.MemberStatus())
	}

	if p.cluster != nil {
		p.cluster.Logger().Debug("Update cluster topology",
			slog.String("provider", "natsstream"),
			slog.Int("members", len(members)))
		p.cluster.MemberList.UpdateClusterTopology(members)

		if p.metricsEnabled {
			p.providerMetrics.TopologyUpdateDuration.Record(
				context.Background(),
				time.Since(start).Seconds(),
				metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
			)
		}
	}
}

// setRole updates the role and notifies listeners.
func (p *Provider) setRole(role cluster.RoleType) {
	p.roleMu.Lock()
	defer p.roleMu.Unlock()

	if role == p.role {
		return
	}

	p.logger().Info("Role changed",
		slog.String("provider", "natsstream"),
		slog.String("from", p.role.String()),
		slog.String("to", role.String()))

	p.role = role

	// Non-blocking send to role changed channel
	select {
	case p.roleChangedChan <- role:
	default:
	}

	// Notify all registered singleton schedulers
	for _, scheduler := range p.schedulers {
		cluster.SafeRunRoleChange(p.logger(), func() {
			scheduler.OnRoleChanged(role)
		})
	}
}

// startRoleChangedNotifyLoop starts the goroutine that notifies the
// external role changed listener.
func (p *Provider) startRoleChangedNotifyLoop() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case role := <-p.roleChangedChan:
				if lis := p.roleChangedListener; lis != nil {
					cluster.SafeRunRoleChange(p.logger(), func() { lis.OnRoleChanged(role) })
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// logger returns the cluster logger if available, otherwise the default logger.
func (p *Provider) logger() *slog.Logger {
	if p.cluster != nil {
		return p.cluster.Logger()
	}
	return slog.Default()
}

// splitHostPort parses an address string into host and port components.
func splitHostPort(addr string) (host string, port int, err error) {
	if h, p, e := net.SplitHostPort(addr); e != nil {
		if addr != "nonhost" {
			err = e
		}
		host = "nonhost"
		port = -1
	} else {
		host = h
		port, err = strconv.Atoi(p)
	}
	return
}

// subjectMatches checks if a subject starts with the given prefix.
func subjectMatches(subject, prefix string) bool {
	return len(subject) > len(prefix) && subject[:len(prefix)] == prefix
}

// marshalJSON is a convenience wrapper around json.Marshal.
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// unmarshalJSON is a convenience wrapper around json.Unmarshal.
func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
