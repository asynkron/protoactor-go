package etcd

import (
	"github.com/asynkron/protoactor-go/actor"
	"sync"
)

// RoleType 表示节点在集群中的角色
type RoleType int

const (
	// Follower indicates the node is not the leader.
	Follower RoleType = iota
	// Leader indicates the node currently holds leadership.
	Leader
)

func (r RoleType) String() string {
	switch r {
	case Leader:
		return "Leader"
	default:
		return "Follower"
	}
}

// SingletonScheduler 管理在领导节点上运行的单例 actor
type SingletonScheduler struct {
	sync.Mutex
	root  *actor.RootContext
	props []*actor.Props
	pids  []*actor.PID
}

// NewSingletonScheduler creates a new scheduler bound to the given root context.
func NewSingletonScheduler(rc *actor.RootContext) *SingletonScheduler {
	return &SingletonScheduler{root: rc}
}

// FromFunc registers an actor function to run when the node becomes leader.
func (s *SingletonScheduler) FromFunc(f actor.ReceiveFunc) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromFunc(f))
	return s
}

// FromProducer registers an actor producer to run when the node becomes leader.
func (s *SingletonScheduler) FromProducer(f actor.Producer) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromProducer(f))
	return s
}

// OnRoleChanged reacts to leadership changes and spawns or poisons actors accordingly.
func (s *SingletonScheduler) OnRoleChanged(rt RoleType) {
	s.Lock()
	defer s.Unlock()
	switch rt {
	case Follower:
		if len(s.pids) > 0 {
			s.root.Logger().Info("I am follower, poison singleton actors")
			for _, pid := range s.pids {
				s.root.Poison(pid)
			}
			s.pids = nil
		}
	case Leader:
		if len(s.props) > 0 {
			s.root.Logger().Info("I am leader now, start singleton actors")
			s.pids = make([]*actor.PID, len(s.props))
			for i, p := range s.props {
				s.pids[i] = s.root.Spawn(p)
			}
		}
	}
}
