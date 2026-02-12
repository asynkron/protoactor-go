package persistence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIntervalSnapshotStrategy(t *testing.T) {
	s := NewIntervalStrategy(4)

	// Index 0 should trigger (0 % 4 == 0) — matches legacy behavior.
	assert.True(t, s.ShouldSnapshot(nil, 0), "index 0 should trigger snapshot")

	// Indices 1, 2, 3 should not trigger.
	assert.False(t, s.ShouldSnapshot(nil, 1))
	assert.False(t, s.ShouldSnapshot(nil, 2))
	assert.False(t, s.ShouldSnapshot(nil, 3))

	// Index 4 should trigger (4 % 4 == 0).
	assert.True(t, s.ShouldSnapshot(nil, 4), "index 4 should trigger snapshot")

	// Index 5 should not.
	assert.False(t, s.ShouldSnapshot(nil, 5))

	// Index 8 (2*interval) should trigger.
	assert.True(t, s.ShouldSnapshot(nil, 8), "index 8 should trigger snapshot")
}

func TestIntervalSnapshotStrategy_ZeroIndex(t *testing.T) {
	// With an interval of 5, index 0 should still trigger (legacy compat).
	s := NewIntervalStrategy(5)
	assert.True(t, s.ShouldSnapshot(nil, 0), "index 0 should trigger snapshot")

	// Indices 1-4 should not.
	for i := 1; i < 5; i++ {
		assert.False(t, s.ShouldSnapshot(nil, i), "index %d should not trigger snapshot", i)
	}
}

func TestIntervalSnapshotStrategy_ZeroInterval(t *testing.T) {
	// An interval of 0 means snapshots are disabled — never trigger.
	s := NewIntervalStrategy(0)
	assert.False(t, s.ShouldSnapshot(nil, 0))
	assert.False(t, s.ShouldSnapshot(nil, 1))
	assert.False(t, s.ShouldSnapshot(nil, 100))
}

func TestIntervalSnapshotStrategy_NegativeInterval(t *testing.T) {
	s := NewIntervalStrategy(-1)
	assert.False(t, s.ShouldSnapshot(nil, 0))
	assert.False(t, s.ShouldSnapshot(nil, 1))
}

func TestTimeBasedSnapshotStrategy(t *testing.T) {
	// Use a controllable clock.
	currentTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return currentTime }

	s := &TimeStrategy{
		interval: 100 * time.Millisecond,
		lastTime: currentTime,
		now:      clock,
	}

	// Immediately after creation: not enough time has elapsed.
	assert.False(t, s.ShouldSnapshot(nil, 0), "should not snapshot immediately")

	// Advance time by 50ms — still not enough.
	currentTime = currentTime.Add(50 * time.Millisecond)
	assert.False(t, s.ShouldSnapshot(nil, 1), "should not snapshot at 50ms")

	// Advance time to 100ms — should trigger.
	currentTime = currentTime.Add(50 * time.Millisecond)
	assert.True(t, s.ShouldSnapshot(nil, 2), "should snapshot at 100ms")

	// Right after snapshot, should not trigger again (timer reset).
	assert.False(t, s.ShouldSnapshot(nil, 3), "should not snapshot right after reset")

	// Advance another 100ms — should trigger again.
	currentTime = currentTime.Add(100 * time.Millisecond)
	assert.True(t, s.ShouldSnapshot(nil, 4), "should snapshot after another 100ms")
}
