package shared

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/asynkron/protoactor-go/cluster"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// SubjectBindingsKey is the gossip key used to share NATS subject bindings
// across cluster members.
const SubjectBindingsKey = "nats-subject-bindings"

// LocalAffinityStrategy implements cluster.MemberStrategy.
// It prefers members whose bound NATS subjects match the actor identity,
// falling back to rendezvous hashing when no affinity match is found.
type LocalAffinityStrategy struct {
	mu      sync.RWMutex
	cluster *cluster.Cluster
	members cluster.Members
	rdv     *cluster.Rendezvous
	rr      *cluster.SimpleRoundRobin
	// Maps member address to list of bound subject patterns
	bindings map[string][]string
}

// NewLocalAffinityStrategy creates a new LocalAffinityStrategy for the given cluster.
func NewLocalAffinityStrategy(c *cluster.Cluster) cluster.MemberStrategy {
	s := &LocalAffinityStrategy{
		cluster:  c,
		members:  make(cluster.Members, 0),
		rdv:      cluster.NewRendezvous(),
		bindings: make(map[string][]string),
	}
	s.rr = cluster.NewSimpleRoundRobin(s)
	return s
}

// GetAllMembers returns all known members.
func (s *LocalAffinityStrategy) GetAllMembers() cluster.Members {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.members
}

// AddMember adds a member and updates the rendezvous hash ring.
func (s *LocalAffinityStrategy) AddMember(member *cluster.Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = append(s.members, member)
	s.rdv.UpdateMembers(s.members)
}

// RemoveMember removes a member and cleans up its bindings.
func (s *LocalAffinityStrategy) RemoveMember(member *cluster.Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.members {
		if m.Address() == member.Address() {
			s.members = append(s.members[:i], s.members[i+1:]...)
			break
		}
	}
	delete(s.bindings, member.Address())
	s.rdv.UpdateMembers(s.members)
}

// GetPartition returns the address of the member that should handle the given key.
// The key is in the format "kind/identity". The strategy first checks if any
// member has bound NATS subjects that prefix-match the identity. If a match is
// found, that member is preferred. Otherwise, rendezvous hashing is used as fallback.
func (s *LocalAffinityStrategy) GetPartition(key string) string {
	// Parse "kind/identity" to extract the identity
	identity := key
	if idx := strings.Index(key, "/"); idx >= 0 {
		identity = key[idx+1:]
	}

	// Refresh bindings from gossip state
	s.refreshBindings()

	s.mu.RLock()
	defer s.mu.RUnlock()

	var bestAddr string
	bestScore := 0

	for _, member := range s.members {
		addr := member.Address()
		patterns, ok := s.bindings[addr]
		if !ok {
			continue
		}
		score := matchScore(identity, patterns)
		if score > bestScore {
			bestScore = score
			bestAddr = addr
		}
	}

	if bestAddr != "" {
		return bestAddr
	}

	// Fall back to rendezvous hashing
	return s.rdv.GetByIdentity(key)
}

// GetActivator returns a member address for activation using round-robin.
func (s *LocalAffinityStrategy) GetActivator(senderAddress string) string {
	return s.rr.GetByRoundRobin()
}

// refreshBindings reads the gossip state to update member subject bindings.
func (s *LocalAffinityStrategy) refreshBindings() {
	state, err := s.cluster.Gossip.GetState(SubjectBindingsKey)
	if err != nil {
		s.cluster.Logger().Warn("Failed to get gossip state for subject bindings",
			slog.Any("error", err))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build a map from member ID to member address
	idToAddr := make(map[string]string, len(s.members))
	for _, m := range s.members {
		idToAddr[m.Id] = m.Address()
	}

	// Update bindings from gossip state
	for memberID, gkv := range state {
		addr, ok := idToAddr[memberID]
		if !ok {
			continue
		}
		if gkv.Value == nil {
			continue
		}
		var sv wrapperspb.StringValue
		if err := gkv.Value.UnmarshalTo(&sv); err != nil {
			s.cluster.Logger().Warn("Failed to unmarshal subject binding",
				slog.String("memberID", memberID),
				slog.Any("error", err))
			continue
		}
		subjects := strings.Split(sv.Value, ",")
		trimmed := make([]string, 0, len(subjects))
		for _, sub := range subjects {
			sub = strings.TrimSpace(sub)
			if sub != "" {
				trimmed = append(trimmed, sub)
			}
		}
		s.bindings[addr] = trimmed
	}
}

// matchScore returns the length of the longest matching prefix between the
// identity and any of the given patterns. Patterns use NATS-style ">" wildcard
// suffix (e.g., "sensors.building-a.>"). Returns 0 if no match is found.
func matchScore(identity string, patterns []string) int {
	best := 0
	for _, pattern := range patterns {
		prefix := strings.TrimSuffix(pattern, ".>")
		if strings.HasPrefix(identity, prefix) {
			if len(prefix) > best {
				best = len(prefix)
			}
		}
	}
	return best
}
