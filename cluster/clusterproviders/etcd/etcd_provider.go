// Package etcd provides an etcd-based cluster provider.
package etcd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Provider uses etcd for cluster membership discovery.
type Provider struct {
	leaseID       clientv3.LeaseID
	cluster       *cluster.Cluster
	baseKey       string
	clusterName   string
	deregistered  bool
	shutdown      bool
	self          *Node
	members       map[string]*Node // all, contains self.
	clusterError  error
	client        *clientv3.Client
	cancelWatch   func()
	cancelWatchCh chan bool
	keepAliveTTL  time.Duration
	retryInterval time.Duration
	revision      uint64
	// deregisterCritical time.Duration
	schedulers          []*SingletonScheduler
	role                RoleType
	roleChangedChan     chan RoleType
	roleChangedListener RoleChangedListener
}

// New creates a provider with default etcd configuration.
func New() (*Provider, error) {
	return NewWithConfig("/protoactor", clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second * 5,
	})
}

// NewWithConfig creates a provider using the specified etcd configuration.
func NewWithConfig(baseKey string, cfg clientv3.Config, opts ...Option) (*Provider, error) {
	c := defaultConfig()
	WithBaseKey(baseKey)(c)
	WithEtcdConfig(cfg)(c)
	for _, opt := range opts {
		opt(c)
	}
	if c.KeepAliveTTL <= 0 {
		c.KeepAliveTTL = defaultKeepAliveTTL
	}
	if c.RetryInterval <= 0 {
		c.RetryInterval = defaultRetryInterval
	}
	if c.client == nil {
		var err error
		if c.client, err = clientv3.New(c.cfg); err != nil {
			return nil, err
		}
	}
	p := &Provider{
		client:              c.client,
		keepAliveTTL:        c.KeepAliveTTL,
		retryInterval:       c.RetryInterval,
		baseKey:             c.BaseKey,
		members:             map[string]*Node{},
		cancelWatchCh:       make(chan bool),
		role:                Follower,
		roleChangedChan:     make(chan RoleType, 1),
		roleChangedListener: c.RoleChanged,
	}
	return p, nil
}

func (p *Provider) init(c *cluster.Cluster) error {
	p.cluster = c
	addr := p.cluster.ActorSystem.Address()
	host, port, err := splitHostPort(addr)
	if err != nil {
		return err
	}

	p.cluster = c
	p.clusterName = p.cluster.Config.Name
	memberID := p.cluster.ActorSystem.ID
	knownKinds := c.GetClusterKinds()
	nodeName := fmt.Sprintf("%v@%v", p.clusterName, memberID)
	p.self = NewNode(nodeName, host, port, knownKinds)
	p.self.SetMeta("id", p.getID())
	return nil
}

// StartMember registers the node in etcd and starts watching for updates.
func (p *Provider) StartMember(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}
	// register self
	if err := p.registerService(); err != nil {
		return err
	}

	p.startRoleChangedNotifyLoop()
	// fetch memberlist
	nodes, err := p.fetchNodes()
	if err != nil {
		return err
	}
	// initialize members
	p.updateNodesWithSelf(nodes)
	p.publishClusterTopologyEvent()
	p.startWatching()

	ctx := context.TODO()
	p.startKeepAlive(ctx)
	p.updateLeadership()
	return nil
}

// StartClient initializes the provider without registering the node.
func (p *Provider) StartClient(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}
	nodes, err := p.fetchNodes()
	if err != nil {
		return err
	}
	// initialize members
	p.updateNodes(nodes)
	p.publishClusterTopologyEvent()
	p.startWatching()
	return nil
}

// Shutdown deregisters the node and stops background tasks.
func (p *Provider) Shutdown(_ bool) error {
	p.shutdown = true
	if !p.deregistered {
		p.updateLeadership()
		err := p.deregisterService()
		if err != nil {
			p.cluster.Logger().Error("deregisterMember", slog.Any("error", err))
			return err
		}
		p.deregistered = true
	}
	if p.cancelWatch != nil {
		p.cancelWatch()
		p.cancelWatch = nil
	}
	return nil
}

func (p *Provider) keepAliveForever(_ context.Context) error {
	if p.self == nil {
		return fmt.Errorf("keepalive must be after initialize")
	}

	data, err := p.self.Serialize()
	if err != nil {
		return err
	}
	fullKey := p.getEtcdKey()

	var leaseID clientv3.LeaseID
	leaseID, err = p.newLeaseID()
	if err != nil {
		return err
	}
	p.setLeaseID(leaseID)

	if leaseID <= 0 {
		return fmt.Errorf("grant lease failed. leaseID=%d", leaseID)
	}
	_, err = p.client.Put(context.TODO(), fullKey, string(data), clientv3.WithLease(leaseID))
	if err != nil {
		return err
	}
	kaRespCh, err := p.client.KeepAlive(context.TODO(), leaseID)
	if err != nil {
		return err
	}

	for resp := range kaRespCh {
		if resp == nil {
			return fmt.Errorf("keep alive failed. resp=%s", resp.String())
		}
		// plog.Infof("keep alive %s ttl=%d", p.getID(), resp.TTL)
		if p.shutdown {
			return nil
		}
	}
	return nil
}

func (p *Provider) startKeepAlive(ctx context.Context) {
	go func() {
		for !p.shutdown {
			if err := ctx.Err(); err != nil {
				p.cluster.Logger().Info("Keepalive was stopped.", slog.Any("error", err))
				return
			}

			if err := p.keepAliveForever(ctx); err != nil {
				p.cluster.Logger().Info("Failure refreshing service TTL. ReTrying...", slog.Duration("after", p.retryInterval), slog.Any("error", err))
			}
			time.Sleep(p.retryInterval)
		}
	}()
}

func (p *Provider) getID() string {
	return p.self.ID
}

func (p *Provider) getEtcdKey() string {
	return p.buildKey(p.clusterName, p.getID())
}

func (p *Provider) registerService() error {
	data, err := p.self.Serialize()
	if err != nil {
		return err
	}
	fullKey := p.getEtcdKey()
	if err != nil {
		return err
	}
	leaseID := p.getLeaseID()
	if leaseID <= 0 {
		_leaseID, err := p.newLeaseID()
		if err != nil {
			return err
		}
		leaseID = _leaseID
		p.setLeaseID(leaseID)
	}
	_, err = p.client.Put(context.TODO(), fullKey, string(data), clientv3.WithLease(leaseID))
	if err != nil {
		return err
	}
	return nil
}

func (p *Provider) deregisterService() error {
	fullKey := p.getEtcdKey()
	_, err := p.client.Delete(context.TODO(), fullKey)
	return err
}

func (p *Provider) handleWatchResponse(resp clientv3.WatchResponse) map[string]*Node {
	changes := map[string]*Node{}
	for _, ev := range resp.Events {
		key := string(ev.Kv.Key)
		nodeID, err := getNodeID(key, "/")
		if err != nil {
			p.cluster.Logger().Error("Invalid member.", slog.String("key", key))
			continue
		}

		switch ev.Type {
		case clientv3.EventTypePut:
			node, err := NewNodeFromBytes(ev.Kv.Value)
			if err != nil {
				p.cluster.Logger().Error("Invalid member.", slog.String("key", key))
				continue
			}
			if p.self.Equal(node) {
				p.cluster.Logger().Debug("Skip self.", slog.String("key", key))
				continue
			}
			if _, ok := p.members[nodeID]; ok {
				p.cluster.Logger().Debug("Update member.", slog.String("key", key))
			} else {
				p.cluster.Logger().Debug("New member.", slog.String("key", key))
			}
			changes[nodeID] = node
			node.SetMeta(metaKeySeq, nodeID)
			node.SetMeta(metaKeySeq, fmt.Sprintf("%d", ev.Kv.Lease))
		case clientv3.EventTypeDelete:
			node, ok := p.members[nodeID]
			if !ok {
				continue
			}
			if p.self.Equal(node) {
				p.cluster.Logger().Debug("Skip self.", slog.String("key", key))
				continue
			}
			p.cluster.Logger().Debug("Delete member.", slog.String("key", key))
			cloned := *node
			cloned.SetAlive(false)
			changes[nodeID] = &cloned
			node.SetMeta(metaKeySeq, nodeID)
			node.SetMeta(metaKeySeq, fmt.Sprintf("%d", ev.Kv.Lease))
		default:
			p.cluster.Logger().Error("Invalid etcd event.type.", slog.String("key", key),
				slog.String("type", ev.Type.String()))
		}
	}
	p.revision = uint64(resp.Header.GetRevision())
	return changes
}

func (p *Provider) keepWatching(ctx context.Context) error {
	clusterKey := p.buildKey(p.clusterName)
	stream := p.client.Watch(ctx, clusterKey, clientv3.WithPrefix())
	return p._keepWatching(stream)
}

func (p *Provider) _keepWatching(stream clientv3.WatchChan) error {
	for resp := range stream {
		if err := resp.Err(); err != nil {
			p.cluster.Logger().Error("Failure watching service.")
			return err
		}
		if len(resp.Events) <= 0 {
			p.cluster.Logger().Error("Empty etcd.events.", slog.Int("events", len(resp.Events)))
			continue
		}
		nodesChanges := p.handleWatchResponse(resp)
		p.updateNodesWithChanges(nodesChanges)
		p.updateLeadership()
		p.publishClusterTopologyEvent()
	}
	return nil
}

func (p *Provider) startWatching() {
	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)
	p.cancelWatch = cancel
	go func() {
		//recover
		defer func() {
			if r := recover(); r != nil {
				p.cluster.Logger().Error("Recovered from panic in keepWatching.", slog.Any("error", r))
				p.clusterError = fmt.Errorf("keepWatching panic: %v", r)
			}
			if p.cancelWatchCh != nil {
				close(p.cancelWatchCh)
			}
			p.cancelWatch = nil
		}()
		for !p.shutdown {
			if err := p.keepWatching(ctx); err != nil {
				p.cluster.Logger().Error("Failed to keepWatching.", slog.Any("error", err))
				p.clusterError = err
			}
		}
	}()
}

// GetHealthStatus returns an error if the cluster health status has problems
func (p *Provider) GetHealthStatus() error {
	return p.clusterError
}

func (p *Provider) buildKey(names ...string) string {
	return strings.Join(append([]string{p.baseKey}, names...), "/")
}

func (p *Provider) fetchNodes() ([]*Node, error) {
	key := p.buildKey(p.clusterName)
	resp, err := p.client.Get(context.TODO(), key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var nodes []*Node
	for _, v := range resp.Kvs {
		n := Node{}
		if err := n.Deserialize(v.Value); err != nil {
			return nil, err
		}
		nodes = append(nodes, &n)
		if v.Lease > 0 {
			n.SetMeta(metaKeySeq, fmt.Sprintf("%d", v.Lease))
			n.SetMeta(metaKeyID, n.ID)
		}
	}
	p.revision = uint64(resp.Header.GetRevision())
	// plog.Debug("fetch nodes",
	// 	log.Uint64("raft term", resp.Header.GetRaftTerm()),
	// 	log.Int64("revision", resp.Header.GetRevision()))
	return nodes, nil
}

func (p *Provider) updateNodes(members []*Node) {
	for _, n := range members {
		p.members[n.ID] = n
	}
}

func (p *Provider) updateNodesWithSelf(members []*Node) {
	p.updateNodes(members)
	p.members[p.self.ID] = p.self
}

func (p *Provider) updateNodesWithChanges(changes map[string]*Node) {
	for memberID, member := range changes {
		p.members[memberID] = member
		if !member.IsAlive() {
			delete(p.members, memberID)
		}
	}
}

func (p *Provider) createClusterTopologyEvent() []*cluster.Member {
	res := make([]*cluster.Member, len(p.members))
	i := 0
	for _, m := range p.members {
		res[i] = m.MemberStatus()
		i++
	}
	return res
}

func (p *Provider) publishClusterTopologyEvent() {
	res := p.createClusterTopologyEvent()
	p.cluster.Logger().Info("Update cluster.", slog.Int("members", len(res)))
	// for _, m := range res {
	// 	plog.Info("\t", log.Object("member", m))
	// }
	p.cluster.MemberList.UpdateClusterTopology(res)
	// p.cluster.ActorSystem.EventStream.Publish(res)
}

func (p *Provider) getLeaseID() clientv3.LeaseID {
	return (clientv3.LeaseID)(atomic.LoadInt64((*int64)(&p.leaseID)))
}

func (p *Provider) setLeaseID(leaseID clientv3.LeaseID) {
	atomic.StoreInt64((*int64)(&p.leaseID), (int64)(leaseID))
	p.self.SetMeta(metaKeySeq, fmt.Sprintf("%d", leaseID))
}

func (p *Provider) newLeaseID() (clientv3.LeaseID, error) {
	ttlSecs := int64((p.keepAliveTTL + time.Second - 1) / time.Second)
	if ttlSecs < 1 {
		ttlSecs = 1
	}
	resp, err := p.client.Grant(context.TODO(), ttlSecs)
	if err != nil {
		return 0, err
	}
	return resp.ID, nil
}

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

// RegisterSingletonScheduler adds a singleton scheduler to be notified on role changes.
func (p *Provider) RegisterSingletonScheduler(scheduler *SingletonScheduler) {
	p.schedulers = append(p.schedulers, scheduler)
}

// 修改现有的角色变化处理逻辑
func (p *Provider) updateLeadership() {
	role := Follower
	ns, err := p.fetchNodes()
	if err != nil {
		p.cluster.Logger().Error("Failed to fetch nodes in updateLeadership.", slog.Any("error", err))
	}

	if p.isLeaderOf(ns) {
		role = Leader
	}
	if role != p.role {
		p.cluster.Logger().Info("Role changed.", slog.String("from", p.role.String()), slog.String("to", role.String()))
		p.role = role
		p.roleChangedChan <- role

		// 通知所有注册的 SingletonScheduler
		for _, scheduler := range p.schedulers {
			safeRun(p.cluster.Logger(), func() {
				scheduler.OnRoleChanged(role)
			})
		}
	}
}

func safeRun(logger *slog.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("OnRoleChanged.", slog.Any("error", fmt.Errorf("%v\n%s", r, string(getRunTimeStack()))))
		}
	}()
	fn()
}

func getRunTimeStack() []byte {
	const size = 64 << 10
	buf := make([]byte, size)
	return buf[:runtime.Stack(buf, false)]
}

func (p *Provider) isLeaderOf(ns []*Node) bool {
	if ns == nil {
		return false
	}
	if len(ns) == 1 && p.self != nil && ns[0].ID == p.self.ID {
		return true
	}
	var minSeq int
	for _, node := range ns {
		if !node.IsAlive() {
			continue
		}
		if seq := node.GetSeq(); (seq > 0 && seq < minSeq) || minSeq == 0 {
			minSeq = seq
		}
	}
	//for _, node := range ns {
	//	if p.self != nil && node.ID == p.self.ID {
	//		return minSeq > 0 && minSeq == p.self.GetSeq()
	//	}
	//}
	if minSeq <= 0 { // 没有在线actor
		return true
	}
	if p.self != nil && p.self.GetSeq() <= minSeq {
		return true
	}
	return false
}

func (p *Provider) startRoleChangedNotifyLoop() {
	go func() {
		for !p.shutdown {
			role := <-p.roleChangedChan
			if lis := p.roleChangedListener; lis != nil {
				safeRun(p.cluster.Logger(), func() { lis.OnRoleChanged(role) })
			}
		}
	}()
}
