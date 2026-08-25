package natskv

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
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

	// The previous release's per-member tracking records are fanned out from
	// here, and that placement is load-bearing rather than convenient.
	//
	// Setup cannot do it: Cluster.StartMember calls IdentityLookup.Setup
	// BEFORE ClusterProvider.StartMember (cluster/cluster.go), and leader
	// election runs inside the latter, so at Setup time IsLeader() is false on
	// every node -- a one-shot leader check started from Setup would migrate
	// nothing, on every node, forever. This loop is the first leader-gated
	// thing that exists, it is already off the startup path, and it already
	// stops on janitorStop, so the migration inherits all three properties and
	// reuses this gate instead of growing a second leadership check.
	//
	// It runs at most once per pass and stops for good once the bucket reports
	// clean, so the steady-state sweep cost is untouched. If the janitor is
	// disabled (JanitorInterval <= 0) the migration does not run at all, and
	// that is safe: the read path returns the union of both shapes for this
	// whole release, so an un-migrated bucket is correct, merely fatter.
	//
	// The latch is per-goroutine and never re-arms, and that has one honest
	// consequence during a mixed-version fleet: a legacy record written by a
	// still-old node AFTER this leader's first clean pass is BRIDGED but not
	// FANNED OUT -- every reader still returns it, because memberTracking and
	// ListGrains union both shapes for the whole release, but this leader will
	// not migrate it until its janitor goroutine is replaced (leadership
	// change, or process restart). The alternative -- re-arming -- would buy a
	// bucket enumeration on every tick, forever, to chase a window that closes
	// when the last old node leaves. A pass is only "done" if the bucket really
	// is free of legacy records: a failed or CAS-lost purge keeps the pass
	// unfinished (fanOutLegacyMemberRecord), so the latch cannot close over a
	// record this leader saw and failed to remove.
	migrationDone := false

	for {
		select {
		case <-il.janitorStop:
			return
		case <-ticker.C:
			if il.isClient || !il.provider.IsLeader() {
				continue
			}
			if !migrationDone {
				migrationDone = il.migrateLegacyMemberRecords(context.Background())
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
//     AND (b) age-since-first-observation >= ActivationAbsentGrace, and the
//     member is still absent on an authoritative point Get.
//     If the member key is restored between sweeps, ac entry is cleared.
//
// Round-trip budget: TWO enumerations (identities, then members) plus one
// identities Get per key, i.e. 2+N. The membership question used to be a KV Get
// per activation record with a PID, making the sweep 1+2N over a set that
// changes only on topology events; one snapshot answers all of them.
func (il *IdentityLookup) janitorSweep(ctx context.Context, ac *janitorAbsenceClock) {
	start := il.now()

	lister, err := il.identities.ListKeys(ctx)
	if err != nil {
		// A ListKeys failure means the sweep could not run at all -- that is an
		// operational fault, not a quiet no-op, so it is surfaced at Warn (was
		// Debug) and counted as an error sweep.
		il.identityLogger().Warn("natskv janitor: list keys failed", slog.Any("error", err))
		il.recordJanitorSweep("error")
		return
	}

	members := il.memberSnapshot(ctx)

	now := start
	var scanned, lockReaps, activationCleans, sweepErrors int

	for key := range lister.Keys() {
		scanned++

		entry, err := il.identities.Get(ctx, key)
		if err != nil {
			// A per-key Get failure (other than a benign not-found race) is an
			// error observation for this sweep.
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				sweepErrors++
			}
			continue
		}

		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			sweepErrors++
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
				lockReaps++
				il.recordJanitorReap("lock")
				// No absence state to clear: lock records are not tracked in ac.
			}
		} else {
			// Completed activation: is the owning member still in the members
			// bucket? Answered from the ONE snapshot taken before this loop,
			// not from a Get per record.
			if len(members) == 0 {
				// No usable membership answer this sweep. Treat every
				// activation as present: fail closed, so the absence clock
				// does not advance and nothing is reaped on missing
				// information. An empty answer counts as missing, not as "the
				// cluster has no members" -- this sweep runs only on the
				// leader, and a leader is itself a member, so an empty members
				// bucket is a broken observation. Lock reaping is unaffected;
				// it never consults membership.
				ac.clear(key)
				continue
			}

			_, memberPresent := members[rec.MemberID]
			if rec.MemberID == "" {
				memberPresent = false
			}

			if memberPresent {
				// Member key exists -- clear any accumulated absence state.
				ac.clear(key)
			} else {
				// Member key absent: timestamp-gated cleanup.
				firstSeen, count := ac.observe(key, now)
				elapsed := now.Sub(firstSeen)
				if count >= 2 && elapsed >= il.config.ActivationAbsentGrace {
					// Confirm before deleting. The snapshot is a watcher-backed
					// enumeration, and a truncated one surfaces no error -- the
					// provider's reconcileMembers guards its own prune with the
					// same authoritative point Get, for the same reason. Here
					// the stake is a live grain's activation record, so the
					// absence CLOCK may run on the cheap snapshot but the
					// irreversible delete may not. This costs one Get per REAP,
					// which is zero in steady state, not one per record.
					if rec.MemberID != "" && il.provider.MemberKeyExists(ctx, rec.MemberID) {
						il.identityLogger().Debug("natskv janitor: member reappeared on confirmation; not reaping",
							slog.String("key", key),
							slog.String("memberID", rec.MemberID))
						ac.clear(key)
						continue
					}

					il.identityLogger().Info("natskv janitor: reaping activation with absent member",
						slog.String("key", key),
						slog.String("memberID", rec.MemberID),
						slog.Int("observations", count),
						slog.Duration("absentFor", elapsed))
					il.casDelete(ctx, key, entry.Revision(), "janitor/absent-member")
					activationCleans++
					il.recordJanitorReap("activation")
					ac.clear(key)
				}
			}
		}
	}

	durationMs := il.now().Sub(start).Milliseconds()
	workDone := lockReaps > 0 || activationCleans > 0
	// Summary line: Info when the sweep did work or hit errors, Debug when the
	// sweep was entirely quiet (all zeros), so operators see reaps/faults but
	// steady-state sweeps do not spam the log.
	logSummary := il.identityLogger().Debug
	if workDone || sweepErrors > 0 {
		logSummary = il.identityLogger().Info
	}
	logSummary("natskv janitor: sweep complete",
		slog.Int("scanned", scanned),
		slog.Int("lockReaps", lockReaps),
		slog.Int("activationCleans", activationCleans),
		slog.Int("errors", sweepErrors),
		slog.Int64("durationMs", durationMs))

	outcome := "clean"
	if sweepErrors > 0 {
		outcome = "error"
	} else if workDone {
		outcome = "work"
	}
	il.recordJanitorSweep(outcome)
}

// memberSnapshot takes the sweep's one membership enumeration, or reports nil
// when it cannot. Nil is "no answer": the caller must fail closed on it, never
// read it as "every member is gone".
func (il *IdentityLookup) memberSnapshot(ctx context.Context) map[string]struct{} {
	if il.provider == nil {
		return nil
	}

	members, err := il.provider.MemberKeysSnapshot(ctx)
	if err != nil {
		// A member-list failure makes every activation look absent, which
		// would reap the whole bucket. Skip the activation half of this sweep
		// entirely; lock reaping is unaffected because it does not consult
		// membership.
		il.identityLogger().Warn("natskv janitor: member snapshot failed; skipping activation checks",
			slog.Any("error", err))

		return nil
	}

	return members
}

// recordJanitorSweep bumps the sweep-outcome counter, labelled by outcome
// ("clean", "work", or "error"). No-op when metrics are disabled.
func (il *IdentityLookup) recordJanitorSweep(outcome string) {
	if il.provider == nil || !il.provider.metricsEnabled || il.provider.providerMetrics == nil {
		return
	}
	il.provider.providerMetrics.JanitorSweepTotal.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("outcome", outcome)))
}

// recordJanitorReap bumps the reap counter, labelled by type ("lock" or
// "activation"). No-op when metrics are disabled.
func (il *IdentityLookup) recordJanitorReap(reapType string) {
	if il.provider == nil || !il.provider.metricsEnabled || il.provider.providerMetrics == nil {
		return
	}
	il.provider.providerMetrics.JanitorReapTotal.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("type", reapType)))
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
