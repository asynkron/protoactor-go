package zk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/go-zookeeper/zk"
)

var (
	_    cluster.ClusterProvider = new(Provider)
	plog                         = log.New(log.InfoLevel, "[CLU/ZK]")
)

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
		plog.Error("connect zk fail", log.Error(err))
		return nil, err
	}
	if auth := zkCfg.Auth; !auth.isEmpty() {
		if err = conn.AddAuth(auth.Scheme, []byte(auth.Credential)); err != nil {
			plog.Error("auth failure.", log.String("scheme", auth.Scheme), log.String("cred", auth.Credential), log.Error(err))
			return nil, err
		}
	}
	p.conn = conn

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
	knownKinds := c.GetClusterKinds()
	nodeName := fmt.Sprintf("%v@%v:%v", p.clusterName, host, port)
	p.self = NewNode(nodeName, host, port, knownKinds)
	p.self.SetMeta(metaKeyID, p.getID())

	if err = p.createClusterNode(p.getClusterKey()); err != nil {
		return err
	}
	return nil
}

func (p *Provider) StartMember(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		plog.Error("init fail " + err.Error())
		return err
	}

	p.startRoleChangedNotifyLoop()

	// register self
	if err := p.registerService(); err != nil {
		plog.Error("register service fail " + err.Error())
		return err
	}
	plog.Info("StartMember register service.", log.String("node", p.self.ID), log.String("seq", p.self.Meta[metaKeySeq]))

	// fetch member list
	nodes, version, err := p.fetchNodes()
	if err != nil {
		plog.Error("fetch nodes fail " + err.Error())
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
			plog.Error("deregisterMember", log.Error(err))
			return err
		}
		p.deregistered = true
	}
	return nil
}

func (p *Provider) UpdateClusterState(state cluster.ClusterState) error {
	if p.shutdown {
		return fmt.Errorf("shutdowned")
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	value := base64.StdEncoding.EncodeToString(data)
	p.self.SetMeta("state", value)
	return p.registerService()
}

func (p *Provider) getID() string {
	return p.self.ID
}

func (p *Provider) getClusterKey() string {
	return p.buildKey(p.clusterName)
}

func (p *Provider) registerService() error {
	data, err := p.self.Serialize()
	if err != nil {
		plog.Error("registerService Serialize fail.", log.Error(err))
		return err
	}

	path, err := p.createEphemeralChildNode(p.getClusterKey(), data)
	if err != nil {
		plog.Error("createEphemeralChildNode fail.", log.String("node", p.getClusterKey()), log.Error(err))
		return err
	}
	p.fullpath = path
	seq, _ := parseSeq(path)
	p.self.SetMeta(metaKeySeq, intToStr(seq))
	plog.Info("RegisterService.", log.String("id", p.self.ID), log.Int("seq", seq))

	return nil
}

func (p *Provider) createClusterNode(dir string) error {
	if dir == "/" {
		return nil
	}
	exist, _, err := p.conn.Exists(dir)
	if err != nil {
		plog.Error("check exist of node fail", log.String("dir", dir), log.Error(err))
		return err
	}
	if exist {
		return nil
	}
	if err = p.createClusterNode(filepath.Dir(dir)); err != nil {
		return err
	}
	if _, err = p.conn.Create(dir, []byte{}, 0, zk.WorldACL(zk.PermAll)); err != nil {
		plog.Error("create dir node fail", log.String("dir", dir), log.Error(err))
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
	clusterKey := p.buildKey(p.clusterName)
	evtChan, err := p.addWatcher(ctx, clusterKey)
	if err != nil {
		plog.Error("list children fail", log.String("node", clusterKey), log.Error(err))
		return err
	}

	return p._keepWatching(registerSelf, evtChan)
}

func (p *Provider) addWatcher(ctx context.Context, clusterKey string) (<-chan zk.Event, error) {
	_, stat, evtChan, err := p.conn.ChildrenW(clusterKey)
	if err != nil {
		plog.Error("list children fail", log.String("node", clusterKey), log.Error(err))
		return nil, err
	}

	plog.Info("KeepWatching cluster.", log.String("cluster", clusterKey), log.Int("children", int(stat.NumChildren)))
	if !p.isChildrenChanged(ctx, stat) {
		return evtChan, nil
	}

	plog.Info("Chilren changed, wait 1 sec and watch again", log.Int("old_cversion", int(p.revision)), log.Int("new_revison", int(stat.Cversion)))
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
		plog.Error("Failure watching service.", log.Error(err))
		if registerSelf && p.clusterNotContainsSelfPath() {
			plog.Info("Register info lost, register self again")
			p.registerService()
		}
		return err
	}
	nodes, version, err := p.fetchNodes()
	if err != nil {
		plog.Error("Failure fetch nodes when watching service.", log.Error(err))
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
			plog.Error("Failure fetch nodes when watching service.", log.Error(err))
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
	clusterKey := p.buildKey(p.clusterName)
	children, _, err := p.conn.Children(clusterKey)
	return err == nil && !stringContains(mapString(children, func(s string) string {
		return filepath.Join(clusterKey, s)
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
				safeRun(func() { lis.OnRoleChanged(role) })
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
		plog.Info("Role changed.", log.String("from", p.role.String()), log.String("to", role.String()))
		p.roleChangedChan <- role
	}
	p.role = role
}

func (p *Provider) onEvent(evt zk.Event) {
	plog.Debug("Zookeeper event.", log.String("type", evt.Type.String()), log.String("state", evt.State.String()), log.String("path", evt.Path))
	if evt.Type != zk.EventSession {
		return
	}
	switch evt.State {
	case zk.StateConnecting, zk.StateDisconnected, zk.StateExpired:
		if p.role == Leader {
			plog.Info("Role changed.", log.String("from", Leader.String()), log.String("to", Follower.String()))
			p.role = Follower
			p.roleChangedChan <- Follower
		}
	case zk.StateConnected, zk.StateHasSession:
	}
}

func (p *Provider) isLeaderOf(ns []*Node) bool {
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
				plog.Error("Failed to keepWatching.", log.Error(err))
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
	return filepath.Join(append([]string{p.baseKey}, names...)...)
}

func (p *Provider) fetchNodes() ([]*Node, int32, error) {
	key := p.buildKey(p.clusterName)
	children, stat, err := p.conn.Children(key)
	if err != nil {
		plog.Error("FetchNodes fail.", log.String("node", key), log.Error(err))
		return nil, 0, err
	}

	var nodes []*Node
	for _, short := range children {
		long := filepath.Join(key, short)
		value, _, err := p.conn.Get(long)
		if err != nil {
			plog.Error("FetchNodes fail.", log.String("node", long), log.Error(err))
			return nil, stat.Cversion, err
		}
		n := Node{Meta: make(map[string]string)}
		if err := n.Deserialize(value); err != nil {
			plog.Error("FetchNodes Deserialize fail.", log.String("node", long), log.String("val", string(value)), log.Error(err))
			return nil, stat.Cversion, err
		}
		seq, err := parseSeq(long)
		if err != nil {
			plog.Error("FetchNodes parse seq fail.", log.String("node", long), log.String("val", string(value)), log.Error(err))
		} else {
			n.SetMeta(metaKeySeq, intToStr(seq))
		}
		plog.Info("FetchNodes new node.", log.String("id", n.ID), log.String("path", long), log.Int("seq", seq))
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

func (p *Provider) createClusterTopologyEvent() cluster.TopologyEvent {
	res := make(cluster.TopologyEvent, len(p.members))
	i := 0
	for _, m := range p.members {
		res[i] = m.MemberStatus()
		i++
	}
	return res
}

func (p *Provider) publishClusterTopologyEvent() {
	res := p.createClusterTopologyEvent()
	plog.Info("Update cluster.", log.Int("members", len(res)))
	p.cluster.MemberList.UpdateClusterTopology(res, p.revision)
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

func (pro *Provider) createEphemeralChildNode(baseKey string, data []byte) (string, error) {
	acl := zk.WorldACL(zk.PermAll)
	prefix := fmt.Sprintf("%s/actor-", baseKey)
	path := ""
	var err error
	for i := 0; i < 3; i++ {
		path, err = pro.conn.CreateProtectedEphemeralSequential(prefix, data, acl)
		if err == zk.ErrNoNode {
			// Create parent node.
			parts := strings.Split(baseKey, "/")
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
