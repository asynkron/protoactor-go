package natsstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time check that IdentityLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityLookup)(nil)

// IdentityLookup implements cluster.IdentityLookup directly using NATS JetStream
// streams for lock acquisition, activation storage, and member tracking.
type IdentityLookup struct {
	provider              *Provider
	cluster               *cluster.Cluster
	memberID              string
	isClient              bool
	identityStream        jetstream.Stream
	identitySubjectPrefix string // e.g. "identities.<clusterName>"
	config                *config
	semaphore             chan struct{}
	memberKeys            map[string][]string // memberID -> list of identity subject keys
	memberKeysMu          sync.Mutex
}

// activationRecord is the JSON-encoded value stored in the identity stream.
type activationRecord struct {
	LockID     string `json:"lid,omitempty"`
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	MemberID   string `json:"mid,omitempty"`
}

// newIdentityLookup creates a new IdentityLookup associated with the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:   p,
		config:     p.config,
		semaphore:  make(chan struct{}, p.config.MaxConcurrency),
		memberKeys: make(map[string][]string),
	}
}

// Setup initializes the identity lookup with the cluster context, creates the
// identity stream, and subscribes to topology events for member cleanup.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	il.memberID = c.ActorSystem.ID
	il.isClient = isClient

	ctx := context.Background()
	js := il.provider.js
	clusterName := c.Config.Name

	// Use a separate subject namespace to avoid overlapping with the cluster
	// membership stream which uses "<prefix>.>".
	il.identitySubjectPrefix = "identities." + clusterName

	streamName := il.config.identityStreamName(clusterName)
	s, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:              streamName,
		Subjects:          []string{il.identitySubjectPrefix + ".>"},
		MaxMsgsPerSubject: 1,
		Retention:         jetstream.LimitsPolicy,
		Storage:           il.config.Storage,
		Replicas:          il.config.Replicas,
	})
	if err != nil {
		slog.Error("natsstream identity: failed to create identity stream",
			slog.Any("error", err))
		return
	}
	il.identityStream = s

	// Subscribe to ClusterTopology events to clean up when members leave.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				il.removeMemberID(context.Background(), member.Id)
			}
		}
	})
}

// identitySubject returns the NATS subject for a given cluster identity.
func (il *IdentityLookup) identitySubject(ci *cluster.ClusterIdentity) string {
	return il.identitySubjectPrefix + "." + kvKey(ci)
}

// Get resolves a cluster identity to an actor PID.
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	il.acquire()
	defer il.release()

	ctx := context.Background()

	// Step 1: Check for existing activation.
	rec := il.getExistingActivation(ctx, ci)
	if rec != nil {
		return pidFromRecord(rec)
	}

	// Step 2: If client, wait for a member to spawn the actor.
	if il.isClient {
		rec = il.waitForActivation(ctx, ci)
		if rec != nil {
			return pidFromRecord(rec)
		}
		return nil
	}

	// Step 3: Try to acquire the spawn lock.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	if !ok {
		// Another node is spawning. Wait for it.
		rec = il.waitForActivation(ctx, ci)
		if rec != nil {
			return pidFromRecord(rec)
		}
		return nil
	}

	// Step 4: Lock acquired -- spawn and store activation.
	pid := il.spawnActivation(ci, lockID, seq)
	if pid == nil {
		// Spawn failed; purge the lock subject so another node can try.
		subject := il.identitySubject(ci)
		_ = il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject))
		return nil
	}

	return pid
}

// RemovePid removes the activation for a cluster identity, but only if
// the currently stored PID matches the one being removed. This prevents
// a TOCTOU race where a concurrent activation could be wiped out by a
// stale RemovePid call.
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	// If the actor is running locally, do NOT delete its identity record.
	if il.cluster != nil && pid.Address == il.cluster.ActorSystem.Address() {
		_, exists := il.cluster.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
		if exists {
			slog.Debug("natsstream identity: RemovePid skipped, actor is alive locally",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.String("pid", pid.String()))
			return
		}
	}

	ctx := context.Background()
	subject := il.identitySubject(ci)

	rec := il.getExistingActivation(ctx, ci)
	if rec == nil {
		return
	}

	if rec.PidID != pid.Id || rec.PidAddress != pid.Address {
		return
	}

	if rec.MemberID != "" {
		il.removeKeyFromMember(rec.MemberID, subject)
	}

	if err := il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
		slog.Error("natsstream identity: RemovePid purge failed",
			slog.String("subject", subject), slog.Any("error", err))
	}
}

// Shutdown performs cleanup when the cluster is shutting down.
func (il *IdentityLookup) Shutdown() {
	if il.memberID != "" {
		il.removeMemberID(context.Background(), il.memberID)
	}
}

// kvKey converts a ClusterIdentity to a subject-safe key.
// ClusterIdentity.AsKey() returns "kind/identity" but NATS subjects use '.' as delimiter
// and '/' is not valid in subject tokens.
func kvKey(ci *cluster.ClusterIdentity) string {
	return strings.ReplaceAll(ci.AsKey(), "/", ".")
}

// acquire acquires a slot from the concurrency semaphore.
func (il *IdentityLookup) acquire() {
	il.semaphore <- struct{}{}
}

// release releases a slot back to the concurrency semaphore.
func (il *IdentityLookup) release() {
	<-il.semaphore
}

// getExistingActivation looks up the current activation for a cluster identity.
// Returns nil if no activation exists or if the message only contains a lock.
func (il *IdentityLookup) getExistingActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	subject := il.identitySubject(ci)

	msg, err := il.identityStream.GetLastMsgForSubject(ctx, subject)
	if err != nil {
		return nil
	}

	var rec activationRecord
	if err := json.Unmarshal(msg.Data, &rec); err != nil {
		return nil
	}

	// Only return if it's a completed activation (has PID info).
	if rec.PidID == "" || rec.PidAddress == "" {
		return nil
	}

	return &rec
}

// tryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity using CAS publish with expected seq 0 (no prior message).
func (il *IdentityLookup) tryAcquireLock(ctx context.Context, ci *cluster.ClusterIdentity) (lockID string, seq uint64, ok bool) {
	lockID = uuid.New().String()
	subject := il.identitySubject(ci)

	rec := activationRecord{
		LockID: lockID,
	}
	data, err := json.Marshal(&rec)
	if err != nil {
		slog.Error("natsstream identity: tryAcquireLock marshal failed", slog.Any("error", err))
		return "", 0, false
	}

	ack, err := il.provider.js.Publish(ctx, subject, data,
		jetstream.WithExpectLastSequencePerSubject(0))
	if err != nil {
		// Expected when another node already has the lock or activation.
		return "", 0, false
	}

	return lockID, ack.Sequence, true
}

// storeActivation stores a completed activation using CAS publish with the
// expected sequence from the lock creation.
func (il *IdentityLookup) storeActivation(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, lockSeq uint64, memberID, pidAddress, pidID string) error {
	subject := il.identitySubject(ci)

	updated := activationRecord{
		LockID:     "", // Clear the lock.
		PidID:      pidID,
		PidAddress: pidAddress,
		MemberID:   memberID,
	}
	data, err := json.Marshal(&updated)
	if err != nil {
		return fmt.Errorf("natsstream identity: storeActivation marshal: %w", err)
	}

	_, err = il.provider.js.Publish(ctx, subject, data,
		jetstream.WithExpectLastSequencePerSubject(lockSeq))
	if err != nil {
		return fmt.Errorf("natsstream identity: storeActivation publish: %w", err)
	}

	// Track this identity subject under the member.
	il.addKeyToMember(memberID, subject)
	return nil
}

// waitForActivation creates a temporary ordered consumer filtered to the specific
// identity subject and waits for a message with a completed activation record.
func (il *IdentityLookup) waitForActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	subject := il.identitySubject(ci)
	streamName := il.config.identityStreamName(il.cluster.Config.Name)

	waitCtx, cancel := context.WithTimeout(ctx, il.config.LockTTL)
	defer cancel()

	consumer, err := il.provider.js.OrderedConsumer(waitCtx, streamName, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{subject},
	})
	if err != nil {
		slog.Error("natsstream identity: waitForActivation consumer failed",
			slog.String("subject", subject), slog.Any("error", err))
		return nil
	}

	iter, err := consumer.Messages()
	if err != nil {
		slog.Error("natsstream identity: waitForActivation messages failed",
			slog.String("subject", subject), slog.Any("error", err))
		return nil
	}
	defer iter.Stop()

	// Stop the iterator when context expires so iter.Next() unblocks.
	go func() {
		<-waitCtx.Done()
		iter.Stop()
	}()

	for {
		msg, err := iter.Next()
		if err != nil {
			return nil
		}

		var rec activationRecord
		if err := json.Unmarshal(msg.Data(), &rec); err != nil {
			msg.Ack()
			continue
		}

		msg.Ack()

		// Activation is complete when PID is set.
		if rec.PidID != "" && rec.PidAddress != "" {
			return &rec
		}
	}
}

// removeMemberID removes all activations belonging to the given member
// by purging their subjects from the identity stream.
//
// The local memberKeys map only tracks identities spawned by THIS node, so
// when a REMOTE member departs, the map will be empty. In that case we fall
// back to scanning the NATS identity stream for all subjects and purging
// any activation record whose MemberID matches the departed member. This
// ensures stale activations are cleaned up regardless of which node
// originally spawned them.
func (il *IdentityLookup) removeMemberID(ctx context.Context, memberID string) {
	if il.identityStream == nil {
		return
	}

	il.memberKeysMu.Lock()
	keys := il.memberKeys[memberID]
	delete(il.memberKeys, memberID)
	il.memberKeysMu.Unlock()

	// Fast path: the local map has keys for this member (it was spawned locally).
	if len(keys) > 0 {
		for _, subject := range keys {
			if err := il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
				slog.Error("natsstream identity: removeMemberID purge failed",
					slog.String("subject", subject), slog.Any("error", err))
			}
		}
		return
	}

	// Slow path: scan the stream for activations belonging to the departed
	// member. This handles the common case where a remote member departs and
	// the local node has no knowledge of which identities it owned.
	il.purgeActivationsForMember(ctx, memberID)
}

// purgeActivationsForMember scans all subjects in the identity stream and
// purges any activation record whose MemberID matches the given member.
// This is the slow path used when the local memberKeys map has no entries
// for the departed member (i.e., it was a remote member).
func (il *IdentityLookup) purgeActivationsForMember(ctx context.Context, memberID string) {
	// Use SubjectFilter to enumerate all identity subjects in the stream.
	info, err := il.identityStream.Info(ctx, jetstream.WithSubjectFilter(il.identitySubjectPrefix+".>"))
	if err != nil {
		slog.Error("natsstream identity: purgeActivationsForMember stream info failed",
			slog.Any("error", err))
		return
	}

	for subject := range info.State.Subjects {
		msg, err := il.identityStream.GetLastMsgForSubject(ctx, subject)
		if err != nil {
			continue
		}

		var rec activationRecord
		if err := json.Unmarshal(msg.Data, &rec); err != nil {
			continue
		}

		if rec.MemberID == memberID {
			if err := il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
				slog.Error("natsstream identity: purgeActivationsForMember purge failed",
					slog.String("subject", subject),
					slog.String("memberID", memberID),
					slog.Any("error", err))
			} else {
				slog.Info("natsstream identity: purged stale activation for departed member",
					slog.String("subject", subject),
					slog.String("memberID", memberID))
			}
		}
	}
}

// addKeyToMember tracks an identity subject under a member.
func (il *IdentityLookup) addKeyToMember(memberID, subject string) {
	il.memberKeysMu.Lock()
	defer il.memberKeysMu.Unlock()

	keys := il.memberKeys[memberID]
	for _, k := range keys {
		if k == subject {
			return // already tracked
		}
	}
	il.memberKeys[memberID] = append(keys, subject)
}

// removeKeyFromMember removes an identity subject from a member's tracking.
func (il *IdentityLookup) removeKeyFromMember(memberID, subject string) {
	il.memberKeysMu.Lock()
	defer il.memberKeysMu.Unlock()

	keys := il.memberKeys[memberID]
	filtered := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != subject {
			filtered = append(filtered, k)
		}
	}
	il.memberKeys[memberID] = filtered
}

// spawnActivation attempts to spawn an actor for the given cluster identity,
// stores the activation, and populates the PID cache.
func (il *IdentityLookup) spawnActivation(ci *cluster.ClusterIdentity, lockID string, lockSeq uint64) *actor.PID {
	kind, ok := il.cluster.TryGetClusterKind(ci.Kind)
	if !ok {
		slog.Error("natsstream identity: unknown kind",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		return nil
	}

	props := cluster.WithClusterIdentity(kind.Props, ci)
	pid, err := il.cluster.ActorSystem.Root.SpawnNamed(props, ci.Kind+"/"+ci.Identity)
	if err != nil {
		if errors.Is(err, actor.ErrNameExists) && pid != nil {
			// The actor was already spawned locally (e.g., by a supervisor
			// after failover) but the identity stream record was purged.
			// Re-register the existing PID in the identity stream so future
			// lookups resolve without re-spawning.
			slog.Info("natsstream identity: actor already exists locally, re-registering",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.String("pid", pid.String()))
		} else {
			slog.Error("natsstream identity: failed to spawn actor",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.Any("error", err))
			return nil
		}
	}

	// Store the activation (CAS update with sequence from lock creation).
	ctx := context.Background()
	storeErr := il.storeActivation(ctx, ci, lockID, lockSeq, il.memberID, pid.Address, pid.Id)
	if storeErr != nil {
		slog.Error("natsstream identity: failed to store activation",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", storeErr))
		// Poison the spawned actor to prevent orphaned processes.
		il.cluster.ActorSystem.Root.Poison(pid)
		return nil
	}

	// Populate the local PID cache.
	il.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)

	return pid
}

// Compile-time check that IdentityLookup implements cluster.GrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityLookup)(nil)

// ListGrains returns all known grain activations across all members.
func (il *IdentityLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	il.memberKeysMu.Lock()
	allSubjects := make(map[string]string) // subject -> memberID
	for memberID, subjects := range il.memberKeys {
		for _, subject := range subjects {
			allSubjects[subject] = memberID
		}
	}
	il.memberKeysMu.Unlock()

	ctx := context.Background()
	var result []*cluster.GrainInfo
	for subject, memberID := range allSubjects {
		gi := il.lookupSubject(ctx, subject, memberID)
		if gi != nil {
			result = append(result, gi)
		}
	}
	return result, nil
}

// ListGrainsByKind returns grain activations filtered by kind.
func (il *IdentityLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := il.ListGrains()
	if err != nil {
		return nil, err
	}
	var result []*cluster.GrainInfo
	for _, g := range all {
		if g.Kind == kind {
			result = append(result, g)
		}
	}
	return result, nil
}

// ListGrainsByMember returns grain activations owned by a specific member.
func (il *IdentityLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	il.memberKeysMu.Lock()
	subjects := make([]string, len(il.memberKeys[memberID]))
	copy(subjects, il.memberKeys[memberID])
	il.memberKeysMu.Unlock()

	ctx := context.Background()
	var result []*cluster.GrainInfo
	for _, subject := range subjects {
		gi := il.lookupSubject(ctx, subject, memberID)
		if gi != nil {
			result = append(result, gi)
		}
	}
	return result, nil
}

// lookupSubject fetches the activation record for a single identity subject
// and converts it into a GrainInfo.
func (il *IdentityLookup) lookupSubject(ctx context.Context, subject string, memberID string) *cluster.GrainInfo {
	msg, err := il.identityStream.GetLastMsgForSubject(ctx, subject)
	if err != nil {
		return nil
	}

	var rec activationRecord
	if err := json.Unmarshal(msg.Data, &rec); err != nil {
		return nil
	}

	// Skip entries that are only locks (no completed activation).
	if rec.PidID == "" || rec.PidAddress == "" {
		return nil
	}

	// Strip the subject prefix to get "kind.identity".
	key := strings.TrimPrefix(subject, il.identitySubjectPrefix+".")
	kind, identity := cluster.ParseDotSeparatedKey(key)

	return &cluster.GrainInfo{
		Identity: identity,
		Kind:     kind,
		PID:      pidFromRecord(&rec),
		MemberID: memberID,
	}
}

// pidFromRecord converts an activationRecord into an actor.PID.
func pidFromRecord(rec *activationRecord) *actor.PID {
	return actor.NewPID(rec.PidAddress, rec.PidID)
}
