package cluster

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/asynkron/protoactor-go/eventstream"
)


// GossipStrategy selects the member with the fewest active grains for the
// requested kind, using per-kind actor counts from gossip heartbeats.
// When no gossip data is available (cold start), it falls back to round-robin.
type GossipStrategy struct {
	mu          sync.RWMutex
	members     []*Member
	actorCounts map[string]map[string]int64 // memberID -> kind -> count
	counter     atomic.Uint32             // round-robin fallback counter (independent of mutex)

	// ScoreMember is an optional callback that allows custom scoring of members.
	// Lower scores are preferred. When nil, the per-kind actor count is used.
	// The counts map may be nil if no gossip data exists for the member yet.
	ScoreMember func(member *Member, kind string, counts map[string]int64) float64

	eventStream *eventstream.EventStream
	sub         *eventstream.Subscription
	closeOnce   sync.Once
}

var _ ActivatorStrategy = (*GossipStrategy)(nil)

// GossipStrategyOption configures a GossipStrategy.
type GossipStrategyOption func(*GossipStrategy)

// WithScoreMember sets a custom scoring callback.
func WithScoreMember(fn func(member *Member, kind string, counts map[string]int64) float64) GossipStrategyOption {
	return func(s *GossipStrategy) {
		s.ScoreMember = fn
	}
}

// NewGossipStrategy creates a new GossipStrategy that subscribes to the given
// EventStream for gossip heartbeat updates.
func NewGossipStrategy(es *eventstream.EventStream, opts ...GossipStrategyOption) *GossipStrategy {
	s := &GossipStrategy{
		actorCounts: make(map[string]map[string]int64),
		eventStream: es,
	}
	for _, opt := range opts {
		opt(s)
	}
	if es != nil {
		s.sub = es.Subscribe(s.handleEvent)
	}
	return s
}

func (s *GossipStrategy) handleEvent(evt any) {
	update, ok := evt.(*GossipUpdate)
	if !ok {
		return
	}
	if update.Key != HeartbeatKey {
		return
	}
	if update.Value == nil {
		return
	}

	var hb MemberHeartbeat
	if err := update.Value.UnmarshalTo(&hb); err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if hb.ActorStatistics != nil && hb.ActorStatistics.ActorCount != nil {
		counts := make(map[string]int64, len(hb.ActorStatistics.ActorCount))
		for k, v := range hb.ActorStatistics.ActorCount {
			counts[k] = v
		}
		s.actorCounts[update.MemberID] = counts
	} else {
		s.actorCounts[update.MemberID] = make(map[string]int64)
	}
}

func (s *GossipStrategy) GetActivator(ci *ClusterIdentity, senderAddress string) *Member {
	// Hold a single RLock for the entire method to provide a consistent
	// snapshot of both the member list and the actor counts.
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := membersForKind(s.members, ci.Kind)
	if len(candidates) == 0 {
		return nil
	}

	// Cold start fallback: round-robin when no gossip data is available.
	if len(s.actorCounts) == 0 {
		idx := s.counter.Add(1)
		return candidates[int(idx)%len(candidates)]
	}

	var best *Member
	bestScore := math.MaxFloat64

	for _, m := range candidates {
		counts := s.actorCounts[m.Id]
		var score float64
		if s.ScoreMember != nil {
			score = s.ScoreMember(m, ci.Kind, counts)
		} else {
			if counts != nil {
				score = float64(counts[ci.Kind])
			}
		}
		// On tie, the first member in insertion order wins. This is deterministic
		// per-node (same insertion order) but may differ across nodes if members
		// joined in different order. For production fairness, consider adding
		// tie-breaking by member ID if this becomes an issue.
		if score < bestScore {
			bestScore = score
			best = m
		}
	}

	return best
}

func (s *GossipStrategy) AddMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members {
		if m.Id == member.Id {
			return
		}
	}
	s.members = append(s.members, member)
}

func (s *GossipStrategy) RemoveMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = removeMemberByID(s.members, member.Id)
	delete(s.actorCounts, member.Id)
}

func (s *GossipStrategy) Close() {
	s.closeOnce.Do(func() {
		if s.eventStream != nil && s.sub != nil {
			s.eventStream.Unsubscribe(s.sub)
		}
	})
}
