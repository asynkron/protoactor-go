package persistence

import (
	"sync"
	"time"
)

// SnapshotStrategy determines when a snapshot should be taken.
type SnapshotStrategy interface {
	// ShouldSnapshot returns true if a snapshot should be taken for the given
	// event index. The state parameter is provided for strategies that may
	// inspect actor state; it may be nil.
	ShouldSnapshot(state interface{}, eventIndex int) bool
}

// IntervalStrategy triggers a snapshot every N events.
// It matches the legacy behavior: eventIndex % interval == 0.
type IntervalStrategy struct {
	interval int
}

// NewIntervalStrategy creates an IntervalStrategy that triggers a snapshot
// every `interval` events. If interval <= 0 the strategy never triggers.
func NewIntervalStrategy(interval int) *IntervalStrategy {
	return &IntervalStrategy{interval: interval}
}

// ShouldSnapshot returns true when eventIndex is a multiple of the configured
// interval (and interval > 0).
func (s *IntervalStrategy) ShouldSnapshot(_ interface{}, eventIndex int) bool {
	if s.interval <= 0 {
		return false
	}
	return eventIndex%s.interval == 0
}

// TimeStrategy triggers a snapshot after a specified duration has elapsed since
// the last snapshot (or since the strategy was created/reset).
type TimeStrategy struct {
	interval time.Duration
	mu       sync.Mutex
	lastTime time.Time
	now      func() time.Time // for testing
}

// NewTimeStrategy creates a TimeStrategy that triggers a snapshot when at
// least `interval` has elapsed since the last snapshot.
func NewTimeStrategy(interval time.Duration) *TimeStrategy {
	return &TimeStrategy{
		interval: interval,
		lastTime: time.Now(),
		now:      time.Now,
	}
}

// ShouldSnapshot returns true when the configured duration has elapsed since
// the last snapshot was taken (or since creation). When it returns true it
// resets the internal timer.
func (s *TimeStrategy) ShouldSnapshot(_ interface{}, _ int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.now().Sub(s.lastTime) >= s.interval {
		s.lastTime = s.now()
		return true
	}
	return false
}
