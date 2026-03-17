package cluster

import (
	"hash/fnv"
	"sync"
)

// RendezvousStrategy selects members using rendezvous (highest random weight)
// hashing. The same identity deterministically maps to the same member given
// a stable member set, providing cache locality.
type RendezvousStrategy struct {
	mu      sync.RWMutex
	members []*Member
}

var _ ActivatorStrategy = (*RendezvousStrategy)(nil)

// NewRendezvousStrategy creates a new RendezvousStrategy.
func NewRendezvousStrategy() *RendezvousStrategy {
	return &RendezvousStrategy{}
}

func (s *RendezvousStrategy) GetActivator(ci *ClusterIdentity, senderAddress string) *Member {
	s.mu.RLock()
	candidates := membersForKind(s.members, ci.Kind)
	s.mu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}

	key := ci.Kind + "/" + ci.Identity
	var best *Member
	var bestHash uint32

	for _, m := range candidates {
		h := rdvHash(key, m.Address())
		if best == nil || h > bestHash {
			best = m
			bestHash = h
		}
	}

	return best
}

// rdvHash computes a rendezvous hash for the given key and member address.
func rdvHash(key, address string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte(address))
	return h.Sum32()
}

func (s *RendezvousStrategy) AddMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members {
		if m.Id == member.Id {
			return
		}
	}
	s.members = append(s.members, member)
}

func (s *RendezvousStrategy) RemoveMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = removeMemberByID(s.members, member.Id)
}

func (s *RendezvousStrategy) Close() {}
