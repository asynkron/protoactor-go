package natskv

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// janitorAbsenceEntry tracks when an activation key was first observed to have
// an absent member key, and how many sweep observations have been made since.
type janitorAbsenceEntry struct {
	firstSeen time.Time
	count     int
}

// janitorAbsenceClock is a per-janitor tracker for activation-absence observations.
// It is entirely separate from the Get-path absenceClock on IdentityLookup.
// All methods are safe for concurrent use (though the janitor goroutine is serial).
type janitorAbsenceClock struct {
	mu  sync.Mutex
	obs map[string]*janitorAbsenceEntry
}

// observe records that key was observed absent at now. Returns the first-seen
// time and the total observation count (including this one).
func (j *janitorAbsenceClock) observe(key string, now time.Time) (time.Time, int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.obs == nil {
		j.obs = make(map[string]*janitorAbsenceEntry)
	}
	e, ok := j.obs[key]
	if !ok {
		e = &janitorAbsenceEntry{firstSeen: now, count: 0}
		j.obs[key] = e
	}
	e.count++
	return e.firstSeen, e.count
}

// clear removes key from the absence map (member re-appeared or cleanup fired).
func (j *janitorAbsenceClock) clear(key string) {
	j.mu.Lock()
	if j.obs != nil {
		delete(j.obs, key)
	}
	j.mu.Unlock()
}

// runJanitor runs a background goroutine that periodically sweeps the
// identities bucket for stale records. It gates on member (non-client) nodes
// and provider leadership; non-leader sweeps are skipped silently.
func (il *IdentityLookup) runJanitor() {
	if il.config.JanitorInterval <= 0 {
		return
	}

	ac := &janitorAbsenceClock{}
	ticker := time.NewTicker(il.config.JanitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-il.janitorStop:
			return
		case <-ticker.C:
			if il.isClient || !il.provider.IsLeader() {
				continue
			}
			il.janitorSweep(context.Background(), ac)
		}
	}
}

// janitorSweep performs one sweep of the identities bucket and reaps stale records.
//
// Reap rules:
//   - Lock-only record (PidID == "") older than HardReapAge: CAS-delete.
//   - Activation (PidID set) whose member key is absent from the members bucket:
//     record absence via ac; delete only when BOTH (a) >= 2 sweep observations
//     AND (b) age-since-first-observation >= ActivationAbsentGrace.
//     If the member key is restored between sweeps, ac entry is cleared.
func (il *IdentityLookup) janitorSweep(ctx context.Context, ac *janitorAbsenceClock) {
	lister, err := il.identities.ListKeys(ctx)
	if err != nil {
		il.identityLogger().Debug("natskv janitor: list keys failed", slog.Any("error", err))
		return
	}

	now := il.now()

	for key := range lister.Keys() {
		entry, err := il.identities.Get(ctx, key)
		if err != nil {
			continue
		}

		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			continue
		}

		if rec.PidID == "" {
			// Lock-only record: apply the hard-reap-age threshold.
			age := entryAge(entry, now)
			if age > il.config.HardReapAge {
				il.identityLogger().Info("natskv janitor: reaping aged lock",
					slog.String("key", key),
					slog.String("memberID", rec.MemberID),
					slog.Duration("age", age))
				il.casDelete(ctx, key, entry.Revision(), "janitor/hard-reap-lock")
				// No absence state to clear: lock records are not tracked in ac.
			}
		} else {
			// Completed activation: check whether the owning member key exists
			// in the members bucket (direct KV read, not the in-memory list).
			memberPresent := rec.MemberID != "" && il.provider.MemberKeyExists(ctx, rec.MemberID)
			if memberPresent {
				// Member key exists -- clear any accumulated absence state.
				ac.clear(key)
			} else {
				// Member key absent: timestamp-gated cleanup.
				firstSeen, count := ac.observe(key, now)
				elapsed := now.Sub(firstSeen)
				if count >= 2 && elapsed >= il.config.ActivationAbsentGrace {
					il.identityLogger().Info("natskv janitor: reaping activation with absent member",
						slog.String("key", key),
						slog.String("memberID", rec.MemberID),
						slog.Int("observations", count),
						slog.Duration("absentFor", elapsed))
					il.casDelete(ctx, key, entry.Revision(), "janitor/absent-member")
					ac.clear(key)
				}
			}
		}
	}
}

// recordWaitTimeoutOutcome classifies the outcome of a waitForActivation timeout
// and records the metric counter. A follow-up Get on the key determines whether
// the activation completed after the waiter timed out (activated_late) or is
// still a lock-only / absent record (still_locked).
func (il *IdentityLookup) recordWaitTimeoutOutcome(ctx context.Context, key string) {
	if il.provider == nil || !il.provider.metricsEnabled || il.provider.providerMetrics == nil {
		return
	}

	outcome := "still_locked"
	entry, err := il.identities.Get(ctx, key)
	if err == nil {
		var rec activationRecord
		if jsonErr := json.Unmarshal(entry.Value(), &rec); jsonErr == nil && rec.PidID != "" {
			outcome = "activated_late"
		}
	}

	il.provider.providerMetrics.LockWaitTimeoutTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("outcome", outcome)))
}
