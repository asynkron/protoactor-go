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

	var best *Member
	var bestHash uint32

	for _, m := range candidates {
		h := rdvHashIdentity(ci.Kind, ci.Identity, m.Address())
		if best == nil || h > bestHash {
			best = m
			bestHash = h
		}
	}

	return best
}

// rdvHashIdentity computes a rendezvous hash for the given kind, identity,
// and member address. Fields are separated by NUL bytes to prevent hash
// collisions when identity strings contain '/'.
func rdvHashIdentity(kind, identity, address string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(kind))
	_, _ = h.Write([]byte{0x00})
	_, _ = h.Write([]byte(identity))
	_, _ = h.Write([]byte{0x00})
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
