package cluster

import (
	"sync"
	"sync/atomic"
)

// LocalAffinityStrategy prefers the local node for grain placement.
// If the local node does not support the requested kind, it falls back
// to round-robin selection across other members.
type LocalAffinityStrategy struct {
	mu      sync.RWMutex
	members []*Member
	counter uint32
}

var _ ActivatorStrategy = (*LocalAffinityStrategy)(nil)

// NewLocalAffinityStrategy creates a new LocalAffinityStrategy.
func NewLocalAffinityStrategy() *LocalAffinityStrategy {
	return &LocalAffinityStrategy{}
}

func (s *LocalAffinityStrategy) GetActivator(ci *ClusterIdentity, senderAddress string) *Member {
	s.mu.RLock()
	candidates := membersForKind(s.members, ci.Kind)
	s.mu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}

	// Prefer the local node.
	for _, m := range candidates {
		if m.Address() == senderAddress {
			return m
		}
	}

	// Fall back to round-robin.
	idx := atomic.AddUint32(&s.counter, 1)
	return candidates[int(idx)%len(candidates)]
}

func (s *LocalAffinityStrategy) AddMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members {
		if m.Id == member.Id {
			return
		}
	}
	s.members = append(s.members, member)
}

func (s *LocalAffinityStrategy) RemoveMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = removeMemberByID(s.members, member.Id)
}

func (s *LocalAffinityStrategy) Close() {}
