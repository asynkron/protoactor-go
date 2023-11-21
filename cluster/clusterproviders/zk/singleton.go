package zk

import (
	"sync"

	"github.com/asynkron/protoactor-go/actor"
)

type SingletonScheduler struct {
	sync.Mutex
	root  *actor.RootContext
	props []*actor.Props
	pids  []*actor.PID
}

func NewSingletonScheduler(rc *actor.RootContext) *SingletonScheduler {
	return &SingletonScheduler{root: rc}
}

func (s *SingletonScheduler) FromFunc(f actor.ReceiveFunc) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromFunc(f))
	return s
}

func (s *SingletonScheduler) FromProducer(f actor.Producer) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromProducer(f))
	return s
}

func (s *SingletonScheduler) OnRoleChanged(rt RoleType) {

	s.Lock()
	defer s.Unlock()
	if rt == Follower {
		if len(s.pids) > 0 {
			s.root.Logger().Info("I am follower, poison singleton actors")
			for _, pid := range s.pids {
				s.root.Poison(pid)
			}
			s.pids = nil
		}
	} else if rt == Leader {
		if len(s.props) > 0 {
			s.root.Logger().Info("I am leader now, start singleton actors")
			s.pids = make([]*actor.PID, len(s.props))
			for i, p := range s.props {
				s.pids[i] = s.root.Spawn(p)
			}
		}
	}
}
