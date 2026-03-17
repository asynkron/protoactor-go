package cluster

import (
	"sync"
	"sync/atomic"
)

// membersForKind filters a member slice to those that support the given kind.
func membersForKind(members []*Member, kind string) []*Member {
	var result []*Member
	for _, m := range members {
		if m.HasKind(kind) {
			result = append(result, m)
		}
	}
	return result
}

// removeMemberByID removes a member from a slice by ID, returning the new slice.
func removeMemberByID(members []*Member, id string) []*Member {
	result := make([]*Member, 0, len(members))
	for _, m := range members {
		if m.Id != id {
			result = append(result, m)
		}
	}
	return result
}

// RoundRobinStrategy selects members in a cyclic order.
// It is the default ActivatorStrategy.
type RoundRobinStrategy struct {
	mu      sync.RWMutex
	members []*Member
	counter uint32
}

var _ ActivatorStrategy = (*RoundRobinStrategy)(nil)

// NewRoundRobinStrategy creates a new RoundRobinStrategy.
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

func (s *RoundRobinStrategy) GetActivator(ci *ClusterIdentity, senderAddress string) *Member {
	s.mu.RLock()
	candidates := membersForKind(s.members, ci.Kind)
	s.mu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}
	idx := atomic.AddUint32(&s.counter, 1)
	return candidates[int(idx)%len(candidates)]
}

func (s *RoundRobinStrategy) AddMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Avoid duplicates.
	for _, m := range s.members {
		if m.Id == member.Id {
			return
		}
	}
	s.members = append(s.members, member)
}

func (s *RoundRobinStrategy) RemoveMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = removeMemberByID(s.members, member.Id)
}

func (s *RoundRobinStrategy) Close() {}
