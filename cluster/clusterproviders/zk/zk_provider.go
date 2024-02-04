package zk

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/go-zookeeper/zk"
)

var _ cluster.ClusterProvider = new(Provider)

type RoleType int

const (
	Follower RoleType = iota
	Leader
)

func (r RoleType) String() string {
	if r == Leader {
		return "LEADER"
	}
	return "FOLLOWER"
}

type Provider struct {
	cluster             *cluster.Cluster
	baseKey             string
	clusterName         string
	clusterKey          string
	deregistered        bool
	shutdown            bool
	self                *Node
	members             map[string]*Node // all, contains self.
	clusterError        error
	conn                zkConn
	revision            uint64
	fullpath            string
	roleChangedListener RoleChangedListener
	role                RoleType
	roleChangedChan     chan RoleType
}

// New zk cluster provider with config
func New(endpoints []string, opts ...Option) (*Provider, error) {
	zkCfg := defaultConfig()
	withEndpoints(endpoints)(zkCfg)
	for _, fn := range opts {
		fn(zkCfg)
	}
	p := &Provider{
		cluster:             &cluster.Cluster{},
		baseKey:             zkCfg.BaseKey,
		clusterKey:          "",
		clusterName:         "",
		deregistered:        false,
		shutdown:            false,
		self:                &Node{},
		members:             map[string]*Node{},
		revision:            0,
		fullpath:            "",
		roleChangedListener: zkCfg.RoleChanged,
		roleChangedChan:     make(chan RoleType, 1),
		role:                Follower,
	}
	conn, err := connectZk(endpoints, zkCfg.SessionTimeout, WithEventCallback(p.onEvent))
	if err != nil {
		return nil, err
	}
	if auth := zkCfg.Auth; !auth.isEmpty() {
		if err = conn.AddAuth(auth.Scheme, []byte(auth.Credential)); err != nil {
			return nil, err
		}
	}
	p.conn = conn

	return p, nil
}

func (p *Provider) IsLeader() bool {
	return p.role == Leader
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
	p.clusterKey = joinPath(p.baseKey, p.clusterName)
	knownKinds := c.GetClusterKinds()
	nodeName := fmt.Sprintf("%v@%v:%v", p.clusterName, host, port)
	p.self = NewNode(nodeName, host, port, knownKinds)
	p.self.SetMeta(metaKeyID, p.getID())

	if err = p.createClusterNode(p.clusterKey); err != nil {
		return err
	}
	return nil
}

func (p *Provider) StartMember(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		p.cluster.Logger().Error("init fail " + err.Error())
		return err
	}

	p.startRoleChangedNotifyLoop()

	// register self
	if err := p.registerService(); err != nil {
		p.cluster.Logger().Error("register service fail " + err.Error())
		return err
	}
	p.cluster.Logger().Info("StartMember register service.", slog.String("node", p.self.ID), slog.String("seq", p.self.Meta[metaKeySeq]))

	// fetch member list
	nodes, version, err := p.fetchNodes()
	if err != nil {
		p.cluster.Logger().Error("fetch nodes fail " + err.Error())
		return err
	}
	// initialize members
	p.updateNodesWithSelf(nodes, version)
	p.publishClusterTopologyEvent()
	p.updateLeadership(nodes)
	p.startWatching(true)

	return nil
}

func (p *Provider) StartClient(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}
	nodes, version, err := p.fetchNodes()
	if err != nil {
		return err
	}
	// initialize members
	p.updateNodes(nodes, version)
	p.publishClusterTopologyEvent()
	p.startWatching(false)

	return nil
}

func (p *Provider) Shutdown(graceful bool) error {
	p.shutdown = true
	if !p.deregistered {
		p.updateLeadership(nil)
		err := p.deregisterService()
		if err != nil {
			p.cluster.Logger().Error("deregisterMember", slog.Any("error", err))
			return err
		}
		p.deregistered = true
	}
	return nil
}

func (p *Provider) getID() string {
	return p.self.ID
}

func (p *Provider) registerService() error {
	data, err := p.self.Serialize()
	if err != nil {
		p.cluster.Logger().Error("registerService Serialize fail.", slog.Any("error", err))
		return err
	}

	path, err := p.createEphemeralChildNode(data)
	if err != nil {
		p.cluster.Logger().Error("createEphemeralChildNode fail.", slog.String("node", p.clusterKey), slog.Any("error", err))
		return err
	}
	p.fullpath = path
	seq, _ := parseSeq(path)
	p.self.SetMeta(metaKeySeq, intToStr(seq))
	p.cluster.Logger().Info("RegisterService.", slog.String("id", p.self.ID), slog.Int("seq", seq))

	return nil
}

func (p *Provider) createClusterNode(dir string) error {
	if dir == "/" {
		return nil
	}
	exist, _, err := p.conn.Exists(dir)
	if err != nil {
		p.cluster.Logger().Error("check exist of node fail", slog.String("dir", dir), slog.Any("error", err))
		return err
	}
	if exist {
		return nil
	}
	if err = p.createClusterNode(getParentDir(dir)); err != nil {
		return err
	}
	if _, err = p.conn.Create(dir, []byte{}, 0, zk.WorldACL(zk.PermAll)); err != nil {
		p.cluster.Logger().Error("create dir node fail", slog.String("dir", dir), slog.Any("error", err))
		return err
	}
	return nil
}

func (p *Provider) deregisterService() error {
	if p.fullpath != "" {
		p.conn.Delete(p.fullpath, -1)
	}
	p.fullpath = ""
	p.conn.Close()
	return nil
}

func (p *Provider) keepWatching(ctx context.Context, registerSelf bool) error {
	evtChan, err := p.addWatcher(ctx, p.clusterKey)
	if err != nil {
		p.cluster.Logger().Error("list children fail", slog.String("node", p.clusterKey), slog.Any("error", err))
		return err
	}

	return p._keepWatching(registerSelf, evtChan)
}

func (p *Provider) addWatcher(ctx context.Context, clusterKey string) (<-chan zk.Event, error) {
	_, stat, evtChan, err := p.conn.ChildrenW(clusterKey)
	if err != nil {
		p.cluster.Logger().Error("list children fail", slog.String("node", clusterKey), slog.Any("error", err))
		return nil, err
	}

	p.cluster.Logger().Info("KeepWatching cluster.", slog.String("cluster", clusterKey), slog.Int("children", int(stat.NumChildren)))
	if !p.isChildrenChanged(ctx, stat) {
		return evtChan, nil
	}

	p.cluster.Logger().Info("Chilren changed, wait 1 sec and watch again", slog.Int("old_cversion", int(p.revision)), slog.Int("new_revison", int(stat.Cversion)))
	time.Sleep(1 * time.Second)
	nodes, version, err := p.fetchNodes()
	if err != nil {
		return nil, err
	}
	// initialize members
	p.updateNodes(nodes, version)
	p.publishClusterTopologyEvent()
	p.updateLeadership(nodes)
	return p.addWatcher(ctx, clusterKey)
}

func (p *Provider) isChildrenChanged(ctx context.Context, stat *zk.Stat) bool {
	return stat.Cversion != int32(p.revision)
}

func (p *Provider) _keepWatching(registerSelf bool, stream <-chan zk.Event) error {
	event := <-stream
	if err := event.Err; err != nil {
		p.cluster.Logger().Error("Failure watching service.", slog.Any("error", err))
		if registerSelf && p.clusterNotContainsSelfPath() {
			p.cluster.Logger().Info("Register info lost, register self again")
			p.registerService()
		}
		return err
	}
	nodes, version, err := p.fetchNodes()
	if err != nil {
		p.cluster.Logger().Error("Failure fetch nodes when watching service.", slog.Any("error", err))
		return err
	}
	if !p.containSelf(nodes) && registerSelf {
		// i am lost, register self
		if err = p.registerService(); err != nil {
			return err
		}
		// reload nodes
		nodes, version, err = p.fetchNodes()
		if err != nil {
			p.cluster.Logger().Error("Failure fetch nodes when watching service.", slog.Any("error", err))
			return err
		}
	}
	p.updateNodes(nodes, version)
	p.publishClusterTopologyEvent()
	if registerSelf {
		p.updateLeadership(nodes)
	}

	return nil
}

func (p *Provider) clusterNotContainsSelfPath() bool {
	children, _, err := p.conn.Children(p.clusterKey)
	return err == nil && !stringContains(mapString(children, func(s string) string {
		return joinPath(p.clusterKey, s)
	}), p.fullpath)
}

func (p *Provider) containSelf(ns []*Node) bool {
	for _, node := range ns {
		if p.self != nil && node.ID == p.self.ID {
			return true
		}
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

func (p *Provider) updateLeadership(ns []*Node) {
	role := Follower
	if p.isLeaderOf(ns) {
		role = Leader
	}
	if role != p.role {
		p.cluster.Logger().Info("Role changed.", slog.String("from", p.role.String()), slog.String("to", role.String()))
		p.role = role
		p.roleChangedChan <- role
	}
}

func (p *Provider) onEvent(evt zk.Event) {
	if evt.Type != zk.EventSession {
		return
	}
	switch evt.State {
	case zk.StateConnecting, zk.StateDisconnected, zk.StateExpired:
		if p.role == Leader {
			p.role = Follower
			p.roleChangedChan <- Follower
		}
	case zk.StateConnected, zk.StateHasSession:
	}
}

func (p *Provider) isLeaderOf(ns []*Node) bool {
	if len(ns) == 1 && p.self != nil && ns[0].ID == p.self.ID {
		return true
	}
	var minSeq int
	for _, node := range ns {
		if seq := node.GetSeq(); (seq > 0 && seq < minSeq) || minSeq == 0 {
			minSeq = seq
		}
	}
	for _, node := range ns {
		if p.self != nil && node.ID == p.self.ID {
			return minSeq > 0 && minSeq == p.self.GetSeq()
		}
	}
	return false
}

func (p *Provider) startWatching(registerSelf bool) {
	ctx := context.TODO()
	go func() {
		for !p.shutdown {
			if err := p.keepWatching(ctx, registerSelf); err != nil {
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

func (p *Provider) fetchNodes() ([]*Node, int32, error) {
	children, stat, err := p.conn.Children(p.clusterKey)
	if err != nil {
		p.cluster.Logger().Error("FetchNodes fail.", slog.String("node", p.clusterKey), slog.Any("error", err))
		return nil, 0, err
	}

	var nodes []*Node
	for _, short := range children {
		long := joinPath(p.clusterKey, short)
		value, _, err := p.conn.Get(long)
		if err != nil {
			p.cluster.Logger().Error("FetchNodes fail.", slog.String("node", long), slog.Any("error", err))
			return nil, stat.Cversion, err
		}
		n := Node{Meta: make(map[string]string)}
		if err := n.Deserialize(value); err != nil {
			p.cluster.Logger().Error("FetchNodes Deserialize fail.", slog.String("node", long), slog.String("val", string(value)), slog.Any("error", err))
			return nil, stat.Cversion, err
		}
		seq, err := parseSeq(long)
		if err != nil {
			p.cluster.Logger().Error("FetchNodes parse seq fail.", slog.String("node", long), slog.String("val", string(value)), slog.Any("error", err))
		} else {
			n.SetMeta(metaKeySeq, intToStr(seq))
		}
		p.cluster.Logger().Info("FetchNodes new node.", slog.String("id", n.ID), slog.String("path", long), slog.Int("seq", seq))
		nodes = append(nodes, &n)
	}
	return p.uniqNodes(nodes), stat.Cversion, nil
}

func (p *Provider) updateNodes(members []*Node, reversion int32) {
	nm := make(map[string]*Node)
	for _, n := range members {
		nm[n.ID] = n
	}
	p.members = nm
	p.revision = uint64(reversion)
}

func (p *Provider) uniqNodes(nodes []*Node) []*Node {
	nodeMap := make(map[string]*Node)
	for _, node := range nodes {
		if n, ok := nodeMap[node.GetAddressString()]; ok {
			// keep node with higher version
			if node.GetSeq() > n.GetSeq() {
				nodeMap[node.GetAddressString()] = node
			}
		} else {
			nodeMap[node.GetAddressString()] = node
		}
	}

	var out []*Node
	for _, node := range nodeMap {
		out = append(out, node)
	}
	return out
}

func (p *Provider) updateNodesWithSelf(members []*Node, version int32) {
	p.updateNodes(members, version)
	p.members[p.self.ID] = p.self
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
	p.cluster.MemberList.UpdateClusterTopology(res)
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

func (pro *Provider) createEphemeralChildNode(data []byte) (string, error) {
	acl := zk.WorldACL(zk.PermAll)
	prefix := joinPath(pro.clusterKey, "actor-")
	path := ""
	var err error
	for i := 0; i < 3; i++ {
		path, err = pro.conn.CreateProtectedEphemeralSequential(prefix, data, acl)
		if err == zk.ErrNoNode {
			// Create parent node.
			parts := strings.Split(pro.clusterKey, "/")
			pth := ""
			for _, p := range parts[1:] {
				var exists bool
				pth += "/" + p
				exists, _, err = pro.conn.Exists(pth)
				if err != nil {
					return "", err
				}
				if exists == true {
					continue
				}
				_, err = pro.conn.Create(pth, []byte{}, 0, acl)
				if err != nil && err != zk.ErrNodeExists {
					return "", err
				}
			}
		} else if err == nil {
			break
		} else {
			return "", err
		}
	}
	return path, err
}
