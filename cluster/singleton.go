package cluster

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	"github.com/awevoke/protoactor-go/actor"
)

// RoleType represents the leadership role of a cluster node.
type RoleType int

const (
	// RoleFollower indicates the node is not the leader.
	RoleFollower RoleType = iota
	// RoleLeader indicates the node currently holds leadership.
	RoleLeader
)

// String returns the human-readable name of the role.
func (r RoleType) String() string {
	switch r {
	case RoleFollower:
		return "Follower"
	case RoleLeader:
		return "Leader"
	default:
		return fmt.Sprintf("RoleType(%d)", int(r))
	}
}

// RoleChangedListener is notified when the cluster node's leadership role changes.
// Implementations must not call back into the provider's RegisterSingletonScheduler
// from within OnRoleChanged — doing so will deadlock.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

// Compile-time check that SingletonScheduler implements RoleChangedListener.
var _ RoleChangedListener = (*SingletonScheduler)(nil)

// SafeRunRoleChange wraps a function call with panic recovery, logging any panic
// that occurs during role change notification.
func SafeRunRoleChange(logger *slog.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 64<<10)
			buf = buf[:runtime.Stack(buf, false)]
			logger.Warn("OnRoleChanged panic recovered",
				slog.Any("error", fmt.Errorf("%v\n%s", r, buf)))
		}
	}()
	fn()
}

// SingletonScheduler manages actors that should run on exactly one node — the leader.
// When the node becomes leader, all registered actors are spawned.
// When the node loses leadership, all running actors are poisoned.
type SingletonScheduler struct {
	sync.Mutex
	root  *actor.RootContext
	props []*actor.Props
	pids  []*actor.PID
}

// NewSingletonScheduler creates a new SingletonScheduler.
func NewSingletonScheduler(rc *actor.RootContext) *SingletonScheduler {
	return &SingletonScheduler{root: rc}
}

// FromFunc registers an actor receive function to run on the leader.
func (s *SingletonScheduler) FromFunc(f actor.ReceiveFunc) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromFunc(f))
	return s
}

// FromProducer registers an actor producer to run on the leader.
func (s *SingletonScheduler) FromProducer(f actor.Producer) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromProducer(f))
	return s
}

// OnRoleChanged is called when the cluster node's leadership role changes.
// On RoleLeader: spawns all registered actors (no-op if already leader with running actors).
// On RoleFollower: poisons all running actors and waits for them to stop before
// returning, so callers (notably provider Shutdown) can rely on the singleton
// being fully stopped once OnRoleChanged returns. The wait is bounded by the
// actor system's StopTimeout.
func (s *SingletonScheduler) OnRoleChanged(rt RoleType) {
	switch rt {
	case RoleFollower:
		s.Lock()
		if len(s.pids) == 0 {
			s.Unlock()
			return
		}
		s.root.Logger().Info("I am follower, poison singleton actors")
		pids := s.pids
		s.pids = nil
		s.Unlock()

		// Poison and wait outside the lock so concurrent registration calls
		// don't block on actor stop. Each future is bounded by the actor
		// system's StopTimeout.
		futures := make([]actor.Future, 0, len(pids))
		for _, pid := range pids {
			futures = append(futures, s.root.PoisonFuture(pid))
		}
		for _, f := range futures {
			_ = f.Wait()
		}
	case RoleLeader:
		s.Lock()
		defer s.Unlock()
		if len(s.pids) > 0 {
			return // already leader with running actors, no-op
		}
		if len(s.props) > 0 {
			s.root.Logger().Info("I am leader now, start singleton actors")
			s.pids = make([]*actor.PID, len(s.props))
			for i, p := range s.props {
				s.pids[i] = s.root.Spawn(p)
			}
		}
	}
}
