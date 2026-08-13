package natskv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/nats-io/nats.go/jetstream"
)

// writeOutcome classifies the result of an identities/tracking-bucket write.
type writeOutcome int

const (
	// writeSuccess: the write applied (nil), or a delete found the key already
	// absent (ErrKeyNotFound on delete). Proves the write path works; resets
	// the failure streak.
	writeSuccess writeOutcome = iota

	// writeCASConflict: a benign optimistic-concurrency conflict. On Create,
	// jetstream.ErrKeyExists; on a guarded Update/Delete, a wrong-last-sequence
	// APIError (JetStream error code 10071). A conflict proves the write PATH
	// works (the server processed the request and rejected it on revision), so
	// it also resets the streak.
	writeCASConflict

	// writeFailure: everything else -- a genuine write-path failure (dead
	// handle, stream churn, connection loss). Increments the streak and is
	// surfaced at Warn.
	writeFailure
)

// classifyWriteError maps a write error to a writeOutcome.
//
// isDelete distinguishes delete sites (where ErrKeyNotFound means the target
// was already gone -- a success) from create/update sites (where the caller
// does not pass a delete and ErrKeyNotFound would be an unexpected failure).
//
// The CAS-conflict test matches the JetStream API error CODE (10071,
// JSErrCodeStreamWrongLastSequence) rather than an error string. Both
// jetstream.ErrKeyExists (returned by Create on an existing key) and the
// wrong-last-sequence error (returned by a revision-guarded Update/Delete)
// carry this code, so a single errors.As on *jetstream.APIError catches both.
func classifyWriteError(err error, isDelete bool) writeOutcome {
	if err == nil {
		return writeSuccess
	}
	if isDelete && errors.Is(err, jetstream.ErrKeyNotFound) {
		return writeSuccess
	}
	var apiErr *jetstream.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode == jetstream.JSErrCodeStreamWrongLastSequence {
		return writeCASConflict
	}
	// jetstream.ErrKeyExists is itself an APIError with code 10071, so the
	// errors.As branch above already covers Create conflicts. This explicit
	// check is a defensive fallback in case the sentinel is ever wrapped in a
	// form that does not expose the APIError.
	if errors.Is(err, jetstream.ErrKeyExists) {
		return writeCASConflict
	}
	return writeFailure
}

// writeFailureGuard tracks the consecutive-non-benign-write-failure streak and
// implements the fail-stop watchdog. All state is guarded by mu; the recorder
// does only O(1) work under the lock and never holds it across a KV call, so it
// does not sit on any hot-path critical section.
type writeFailureGuard struct {
	mu sync.Mutex

	// streak is the number of consecutive writeFailure outcomes. Reset to 0 on
	// any writeSuccess or writeCASConflict.
	streak int
	// streakStart is the time of the first failure in the current streak.
	streakStart time.Time
	// lastErr / lastOp describe the most recent failure, for the fail-stop log.
	lastErr error
	lastOp  string

	// Warn rate-limiting: at most one Warn per warnInterval, carrying the count
	// of failures suppressed since the last emitted Warn.
	lastWarn     time.Time
	suppressed   int
	warnInterval time.Duration

	// tripped ensures FailStop is invoked at most once.
	tripped bool
}

// warn rate-limit interval: at most one Warn line per second.
const writeGuardWarnInterval = time.Second

// recordIdentityWriteOutcome classifies err for the given op and updates the
// failure accounting. It is the single choke point every identities /
// tracking-bucket WRITE site routes through.
//
//   - success / delete-of-absent  -> reset streak, no log.
//   - CAS conflict (code 10071)   -> reset streak, Debug log ("benign").
//   - any other error             -> rate-limited Warn, increment streak,
//     bump the OTel failure counter, and evaluate the fail-stop watchdog.
//
// isDelete tells the classifier whether ErrKeyNotFound is benign (delete) or a
// failure (create/update). It returns the classified outcome so callers can
// keep any control flow that depends on it (none currently need to).
func (il *IdentityLookup) recordIdentityWriteOutcome(op string, isDelete bool, err error) writeOutcome {
	outcome := classifyWriteError(err, isDelete)

	switch outcome {
	case writeSuccess:
		il.writeGuard.reset()
	case writeCASConflict:
		il.writeGuard.reset()
		il.identityLogger().Debug("natskv identity: write CAS conflict (benign)",
			slog.String("op", op),
			slog.Any("error", err))
	case writeFailure:
		il.recordWriteFailure(op, err)
	}
	return outcome
}

// reset clears the failure streak. Called on any benign outcome.
func (g *writeFailureGuard) reset() {
	g.mu.Lock()
	g.streak = 0
	g.streakStart = time.Time{}
	g.mu.Unlock()
}

// recordWriteFailure accounts a genuine write failure: it increments the
// streak, emits a rate-limited Warn, bumps the failure counter, and evaluates
// the fail-stop watchdog. The KV leadership-release + FailStop invocation are
// performed OUTSIDE the guard lock.
func (il *IdentityLookup) recordIdentityWriteFailureCounter(op string) {
	if il.provider == nil || !il.provider.metricsEnabled || il.provider.providerMetrics == nil {
		return
	}
	il.provider.providerMetrics.IdentityWriteFailureTotal.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("op", op)))
}

func (il *IdentityLookup) recordWriteFailure(op string, err error) {
	g := &il.writeGuard

	g.mu.Lock()
	if g.warnInterval == 0 {
		g.warnInterval = writeGuardWarnInterval
	}
	now := il.now()
	if g.streak == 0 {
		g.streakStart = now
	}
	g.streak++
	g.lastErr = err
	g.lastOp = op

	// Rate-limited Warn with suppressed-count.
	emitWarn := false
	suppressed := 0
	if now.Sub(g.lastWarn) >= g.warnInterval {
		emitWarn = true
		suppressed = g.suppressed
		g.suppressed = 0
		g.lastWarn = now
	} else {
		g.suppressed++
	}

	streak := g.streak
	streakStart := g.streakStart
	threshold := il.config.WriteFailureThreshold
	window := il.config.WriteFailureWindow
	shouldTrip := !g.tripped && threshold > 0 &&
		streak >= threshold && now.Sub(streakStart) >= window
	if shouldTrip {
		g.tripped = true
	}
	g.mu.Unlock()

	// Counter and logging happen outside the lock.
	il.recordIdentityWriteFailureCounter(op)

	if emitWarn {
		il.identityLogger().Warn("natskv identity: write failed",
			slog.String("op", op),
			slog.Int("streak", streak),
			slog.Int("suppressed", suppressed),
			slog.Any("error", err))
	}

	if shouldTrip {
		il.tripFailStop(streak, streakStart, err)
	}
}

// tripFailStop performs the fail-stop sequence: one loud Error line, a
// best-effort leadership release (if this node is leader), then invocation of
// the configured FailStop action (default os.Exit(70)). It is only ever
// reached once (guarded by g.tripped).
func (il *IdentityLookup) tripFailStop(streak int, streakStart time.Time, lastErr error) {
	isLeader := il.provider != nil && il.provider.IsLeader()
	window := il.now().Sub(streakStart)

	reason := fmt.Sprintf(
		"identity write path failed: %d consecutive failures over %s (threshold %d, window %s); lastErr=%v; leader=%v",
		streak, window.Round(time.Millisecond), il.config.WriteFailureThreshold,
		il.config.WriteFailureWindow, lastErr, isLeader)

	il.identityLogger().Error("natskv identity: FAIL-STOP triggered by write-path failure",
		slog.Int("streak", streak),
		slog.Duration("failingFor", window),
		slog.Int("threshold", il.config.WriteFailureThreshold),
		slog.Duration("window", il.config.WriteFailureWindow),
		slog.Bool("leader", isLeader),
		slog.Any("lastError", lastErr))

	// If leader, attempt to release the leader key via the (separate, likely
	// still-healthy) leader bucket handle so succession is immediate rather
	// than waiting for LeaderTTL. Best-effort: log the outcome, do not block.
	if isLeader && il.provider != nil {
		released := il.provider.releaseLeadershipBestEffort()
		il.identityLogger().Error("natskv identity: FAIL-STOP leadership release attempted",
			slog.Bool("released", released))
	}

	fn := il.config.FailStop
	if fn == nil {
		fn = defaultFailStop(il.identityLogger())
	}
	fn(reason)
}

// defaultFailStop returns the default fail-stop action: log the reason at
// Error with a CRITICAL marker, then exit the process with failStopExitCode so
// the supervisor restarts it clean and re-establishes the KV write handles.
func defaultFailStop(logger *slog.Logger) func(reason string) {
	return func(reason string) {
		if logger != nil {
			logger.Error("natskv identity: CRITICAL fail-stop, terminating process",
				slog.Int("exitCode", failStopExitCode),
				slog.String("reason", reason))
		}
		os.Exit(failStopExitCode)
	}
}
