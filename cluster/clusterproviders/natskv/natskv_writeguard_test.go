package natskv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wrongLastSeqErr constructs an error carrying the JetStream wrong-last-sequence
// API error code (10071), as returned by a revision-guarded Update/Delete when
// the revision no longer matches.
func wrongLastSeqErr() error {
	return &jetstream.APIError{
		Code:      400,
		ErrorCode: jetstream.JSErrCodeStreamWrongLastSequence,
	}
}

// streakLenForTest exposes the current failure-streak length for assertions.
func (il *IdentityLookup) streakLenForTest() int {
	il.writeGuard.mu.Lock()
	defer il.writeGuard.mu.Unlock()
	return il.writeGuard.streak
}

func TestClassifyWriteError(t *testing.T) {
	otherErr := errors.New("connection reset by peer")

	for _, tc := range []struct {
		name     string
		err      error
		isDelete bool
		want     writeOutcome
	}{
		{"nil success", nil, false, writeSuccess},
		{"delete of absent is success", jetstream.ErrKeyNotFound, true, writeSuccess},
		{"not-found on non-delete is failure", jetstream.ErrKeyNotFound, false, writeFailure},
		{"ErrKeyExists is CAS conflict", jetstream.ErrKeyExists, false, writeCASConflict},
		{"wrong-last-seq (10071) is CAS conflict", wrongLastSeqErr(), false, writeCASConflict},
		{"wrapped wrong-last-seq is CAS conflict", fmt.Errorf("update: %w", wrongLastSeqErr()), false, writeCASConflict},
		{"genuine error is failure", otherErr, false, writeFailure},
		{"wrapped genuine error is failure", fmt.Errorf("x: %w", otherErr), true, writeFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyWriteError(tc.err, tc.isDelete)
			assert.Equal(t, tc.want, got)
		})
	}
}

// newGuardOnlyIL builds an IdentityLookup with only the fields the write guard
// needs: a config, a clock, and a logger source. It has no provider and no KV,
// so the recorder exercises only classification + streak accounting.
func newGuardOnlyIL(t *testing.T, opts ...Option) (*IdentityLookup, *time.Time) {
	t.Helper()
	cfg := newDefaultConfig()
	for _, o := range opts {
		o(cfg)
	}
	nowP := new(time.Time)
	*nowP = time.Unix(1_700_000_000, 0)
	il := &IdentityLookup{
		config: cfg,
		now:    func() time.Time { return *nowP },
	}
	return il, nowP
}

func TestRecordOutcomeStreakAccounting(t *testing.T) {
	realErr := errors.New("kv dead")

	t.Run("success resets streak", func(t *testing.T) {
		il, _ := newGuardOnlyIL(t, WithFailStopDisabled())
		il.recordIdentityWriteOutcome("op", false, realErr)
		il.recordIdentityWriteOutcome("op", false, realErr)
		require.Equal(t, 2, il.streakLenForTest())
		il.recordIdentityWriteOutcome("op", false, nil)
		require.Equal(t, 0, il.streakLenForTest())
	})

	t.Run("CAS conflict resets streak", func(t *testing.T) {
		il, _ := newGuardOnlyIL(t, WithFailStopDisabled())
		il.recordIdentityWriteOutcome("op", false, realErr)
		require.Equal(t, 1, il.streakLenForTest())
		il.recordIdentityWriteOutcome("op", false, wrongLastSeqErr())
		require.Equal(t, 0, il.streakLenForTest(), "benign CAS conflict must reset the streak")
	})

	t.Run("delete-of-absent resets streak", func(t *testing.T) {
		il, _ := newGuardOnlyIL(t, WithFailStopDisabled())
		il.recordIdentityWriteOutcome("op", false, realErr)
		il.recordIdentityWriteOutcome("op", true, jetstream.ErrKeyNotFound)
		require.Equal(t, 0, il.streakLenForTest())
	})

	t.Run("real errors increment", func(t *testing.T) {
		il, _ := newGuardOnlyIL(t, WithFailStopDisabled())
		for i := 0; i < 5; i++ {
			il.recordIdentityWriteOutcome("op", false, realErr)
		}
		require.Equal(t, 5, il.streakLenForTest())
	})
}

func TestFailStopThresholdAndWindow(t *testing.T) {
	realErr := errors.New("kv dead")

	t.Run("trips exactly once at threshold+window", func(t *testing.T) {
		var reasons []string
		var mu sync.Mutex
		il, nowP := newGuardOnlyIL(t,
			WithWriteFailureThreshold(5),
			WithWriteFailureWindow(30*time.Second),
			WithFailStop(func(r string) {
				mu.Lock()
				reasons = append(reasons, r)
				mu.Unlock()
			}),
		)

		// 5 failures but all within the same instant: window not elapsed, no trip.
		for i := 0; i < 5; i++ {
			il.recordIdentityWriteOutcome("casDelete/x", true, realErr)
		}
		require.Empty(t, reasons, "must not trip before window elapses")

		// Advance past the window and fail again: now it trips.
		*nowP = nowP.Add(31 * time.Second)
		il.recordIdentityWriteOutcome("casDelete/x", true, realErr)
		require.Len(t, reasons, 1, "must trip once")
		assert.Contains(t, reasons[0], "identity write path failed")
		assert.Contains(t, reasons[0], "threshold 5")

		// Further failures do not re-trip.
		il.recordIdentityWriteOutcome("casDelete/x", true, realErr)
		require.Len(t, reasons, 1, "must trip at most once")
	})

	t.Run("window not elapsed never trips", func(t *testing.T) {
		var tripped atomic.Int32
		il, _ := newGuardOnlyIL(t,
			WithWriteFailureThreshold(3),
			WithWriteFailureWindow(90*time.Second),
			WithFailStop(func(string) { tripped.Add(1) }),
		)
		for i := 0; i < 50; i++ {
			il.recordIdentityWriteOutcome("op", false, realErr)
		}
		require.Zero(t, tripped.Load(), "50 rapid failures inside window must not trip")
	})

	t.Run("recovery mid-streak resets and prevents trip", func(t *testing.T) {
		var tripped atomic.Int32
		il, nowP := newGuardOnlyIL(t,
			WithWriteFailureThreshold(5),
			WithWriteFailureWindow(30*time.Second),
			WithFailStop(func(string) { tripped.Add(1) }),
		)
		for i := 0; i < 4; i++ {
			il.recordIdentityWriteOutcome("op", false, realErr)
			*nowP = nowP.Add(10 * time.Second)
		}
		// A success recovers the path -- streak resets, streakStart forgotten.
		il.recordIdentityWriteOutcome("op", false, nil)
		require.Equal(t, 0, il.streakLenForTest())

		// New failures build a fresh streak whose window starts now.
		for i := 0; i < 5; i++ {
			il.recordIdentityWriteOutcome("op", false, realErr)
		}
		require.Zero(t, tripped.Load(), "fresh streak within window must not trip")
	})

	t.Run("disabled option never invokes fail-stop", func(t *testing.T) {
		il, nowP := newGuardOnlyIL(t,
			WithWriteFailureThreshold(2),
			WithWriteFailureWindow(time.Second),
			WithFailStopDisabled(),
		)
		il.recordIdentityWriteOutcome("op", false, realErr)
		*nowP = nowP.Add(2 * time.Second)
		il.recordIdentityWriteOutcome("op", false, realErr)
		// The FailStop func is a no-op; the streak still trips (tripped=true)
		// but no process action occurs. Nothing to assert beyond "no panic /
		// no exit"; reaching here means disabled worked.
	})
}

// TestFailStopReleasesLeadershipWhenLeader verifies that when the tripping node
// is leader, the fail-stop path attempts to release the leader key (observable
// by the key disappearing from the leader bucket), and does not when follower.
func TestFailStopReleasesLeadershipWhenLeader(t *testing.T) {
	for _, leader := range []bool{true, false} {
		leader := leader
		t.Run(fmt.Sprintf("leader=%v", leader), func(t *testing.T) {
			srv := startEmbeddedNATS(t)
			nc, js := connectNATS(t, srv)

			p, err := New(nc,
				WithWriteFailureThreshold(2),
				WithWriteFailureWindow(time.Second),
				WithFailStopDisabled(), // don't actually exit
			)
			require.NoError(t, err)
			p.clusterName = "failstop-lead"
			p.ctx = context.Background()
			require.NoError(t, p.createLeaderBucket())

			// Seed the leader key so a release is observable.
			_, err = p.leaderBucket.Put(context.Background(), p.leaderKey(), []byte("me"))
			require.NoError(t, err)
			p.isLeader.Store(leader)

			nowP := new(time.Time)
			*nowP = time.Unix(1_700_000_000, 0)
			il := &IdentityLookup{
				provider: p,
				config:   p.config,
				now:      func() time.Time { return *nowP },
			}

			realErr := errors.New("kv dead")
			il.recordIdentityWriteOutcome("op", false, realErr)
			*nowP = nowP.Add(2 * time.Second)
			il.recordIdentityWriteOutcome("op", false, realErr)

			_, getErr := p.leaderBucket.Get(context.Background(), p.leaderKey())
			if leader {
				require.ErrorIs(t, getErr, jetstream.ErrKeyNotFound,
					"leader key must be released on fail-stop when leader")
				require.False(t, p.IsLeader(), "isLeader must be cleared")
			} else {
				require.NoError(t, getErr, "follower must not touch the leader key")
			}
			_ = js
		})
	}
}

// erroringKV wraps a real jetstream.KeyValue and, when armed, forces the write
// methods (Create/Update/Delete/Put) to return a supplied error. Reads pass
// through to the underlying bucket so ListKeys/Get keep working.
type erroringKV struct {
	jetstream.KeyValue
	mu      sync.Mutex
	failErr error
}

func (e *erroringKV) arm(err error) {
	e.mu.Lock()
	e.failErr = err
	e.mu.Unlock()
}

func (e *erroringKV) armed() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.failErr
}

func (e *erroringKV) Create(ctx context.Context, key string, value []byte, opts ...jetstream.KVCreateOpt) (uint64, error) {
	if err := e.armed(); err != nil {
		return 0, err
	}
	return e.KeyValue.Create(ctx, key, value, opts...)
}

func (e *erroringKV) Update(ctx context.Context, key string, value []byte, revision uint64) (uint64, error) {
	if err := e.armed(); err != nil {
		return 0, err
	}
	return e.KeyValue.Update(ctx, key, value, revision)
}

func (e *erroringKV) Delete(ctx context.Context, key string, opts ...jetstream.KVDeleteOpt) error {
	if err := e.armed(); err != nil {
		return err
	}
	return e.KeyValue.Delete(ctx, key, opts...)
}

func (e *erroringKV) Put(ctx context.Context, key string, value []byte) (uint64, error) {
	if err := e.armed(); err != nil {
		return 0, err
	}
	return e.KeyValue.Put(ctx, key, value)
}

// TestFailStopThroughErroringKV drives the fail-stop path through real write
// sites (casDelete) against an erroring KV wrapper, confirming the streak
// crosses the threshold+window and FailStop fires with a leadership-aware
// reason.
func TestFailStopThroughErroringKV(t *testing.T) {
	base := buildBareIdentityLookup(t)
	ekv := &erroringKV{KeyValue: base.identities}
	base.identities = ekv

	nowP := new(time.Time)
	*nowP = time.Unix(1_700_000_000, 0)
	base.now = func() time.Time { return *nowP }
	base.config.WriteFailureThreshold = 3
	base.config.WriteFailureWindow = 20 * time.Second

	var reason atomic.Value
	base.config.FailStop = func(r string) { reason.Store(r) }

	ekv.arm(errors.New("stream churn: no responders"))

	ctx := context.Background()
	// Below threshold: no trip yet.
	base.casDelete(ctx, "k/1", 1, "test")
	base.casDelete(ctx, "k/1", 1, "test")
	require.Nil(t, reason.Load(), "must not trip below threshold")

	// Advance past the window and fail once more -> trip.
	*nowP = nowP.Add(21 * time.Second)
	base.casDelete(ctx, "k/1", 1, "test")
	require.NotNil(t, reason.Load(), "fail-stop must fire once threshold+window met")
	assert.Contains(t, reason.Load().(string), "identity write path failed")

	// After the KV recovers, a benign delete-of-absent resets nothing new to
	// assert here (already tripped), but must not panic.
	ekv.arm(nil)
	base.casDelete(ctx, "k/absent", 99, "test")
}

// capturingHandler records emitted log records for assertions.
type capturingHandler struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r)
	h.mu.Unlock()
	return nil
}
func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func (h *capturingHandler) countByLevelAndMsg(level slog.Level, sub string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.recs {
		if r.Level == level && strings.Contains(r.Message, sub) {
			n++
		}
	}
	return n
}

// TestWarnRateLimit verifies at most one Warn per second with a suppressed
// count, and that a real error is surfaced at Warn (not swallowed at Debug).
func TestWarnRateLimit(t *testing.T) {
	h := &capturingHandler{}
	// il.cluster is nil, so identityLogger() falls back to slog.Default().
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	nowP := new(time.Time)
	*nowP = time.Unix(1_700_000_000, 0)
	il := &IdentityLookup{
		config: newDefaultConfig(),
		now:    func() time.Time { return *nowP },
	}
	WithFailStopDisabled()(il.config)

	realErr := errors.New("kv dead")
	// 10 failures within the same second -> exactly one Warn.
	for i := 0; i < 10; i++ {
		il.recordIdentityWriteOutcome("casDelete/x", true, realErr)
	}
	require.Equal(t, 1, h.countByLevelAndMsg(slog.LevelWarn, "write failed"),
		"expected a single rate-limited Warn within one second")

	// Advance one second and fail again -> a second Warn, carrying suppressed.
	*nowP = nowP.Add(time.Second)
	il.recordIdentityWriteOutcome("casDelete/x", true, realErr)
	require.Equal(t, 2, h.countByLevelAndMsg(slog.LevelWarn, "write failed"))
}
