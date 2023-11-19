package etcd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	clientv3 "go.etcd.io/etcd/client/v3"
)

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
}

func New() (*Provider, error) {
	return NewWithConfig("/protoactor", clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second * 5,
	})
}

func NewWithConfig(baseKey string, cfg clientv3.Config) (*Provider, error) {
	client, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	p := &Provider{
		client:        client,
		keepAliveTTL:  3 * time.Second,
		retryInterval: 1 * time.Second,
		baseKey:       baseKey,
		members:       map[string]*Node{},
		cancelWatchCh: make(chan bool),
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

func (p *Provider) StartMember(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}

	// fetch memberlist
	nodes, err := p.fetchNodes()
	if err != nil {
		return err
	}
	// initialize members
	p.updateNodesWithSelf(nodes)
	p.publishClusterTopologyEvent()
	p.startWatching()

	// register self
	if err := p.registerService(); err != nil {
		return err
	}
	ctx := context.TODO()
	p.startKeepAlive(ctx)
	return nil
}

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

func (p *Provider) Shutdown(graceful bool) error {
	p.shutdown = true
	if !p.deregistered {
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

func (p *Provider) keepAliveForever(ctx context.Context) error {
	if p.self == nil {
		return fmt.Errorf("keepalive must be after initialize")
	}

	data, err := p.self.Serialize()
	if err != nil {
		return err
	}
	fullKey := p.getEtcdKey()

	var leaseId clientv3.LeaseID
	leaseId, err = p.newLeaseID()
	if err != nil {
		return err
	}
	p.setLeaseID(leaseId)

	if leaseId <= 0 {
		return fmt.Errorf("grant lease failed. leaseId=%d", leaseId)
	}
	_, err = p.client.Put(context.TODO(), fullKey, string(data), clientv3.WithLease(leaseId))
	if err != nil {
		return err
	}
	kaRespCh, err := p.client.KeepAlive(context.TODO(), leaseId)
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
	leaseId := p.getLeaseID()
	if leaseId <= 0 {
		_leaseId, err := p.newLeaseID()
		if err != nil {
			return err
		}
		leaseId = _leaseId
		p.setLeaseID(leaseId)
	}
	_, err = p.client.Put(context.TODO(), fullKey, string(data), clientv3.WithLease(leaseId))
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
		nodeId, err := getNodeID(key, "/")
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
			if _, ok := p.members[nodeId]; ok {
				p.cluster.Logger().Debug("Update member.", slog.String("key", key))
			} else {
				p.cluster.Logger().Debug("New member.", slog.String("key", key))
			}
			changes[nodeId] = node
		case clientv3.EventTypeDelete:
			node, ok := p.members[nodeId]
			if !ok {
				continue
			}
			p.cluster.Logger().Debug("Delete member.", slog.String("key", key))
			cloned := *node
			cloned.SetAlive(false)
			changes[nodeId] = &cloned
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
		p.publishClusterTopologyEvent()
	}
	return nil
}

func (p *Provider) startWatching() {
	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)
	p.cancelWatch = cancel
	go func() {
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

func newContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.TODO(), timeout)
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
	for memberId, member := range changes {
		p.members[memberId] = member
		if !member.IsAlive() {
			delete(p.members, memberId)
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
}

func (p *Provider) newLeaseID() (clientv3.LeaseID, error) {
	ttlSecs := int64(p.keepAliveTTL / time.Second)
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
