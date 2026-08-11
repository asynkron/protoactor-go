package natskv

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	natskvmetrics "github.com/awevoke/protoactor-go/cluster/clusterproviders/natskv/metrics"
)

// setupTestMeterProvider installs a ManualReader-backed MeterProvider so tests
// can collect and inspect recorded metric data points.
func setupTestMeterProvider(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})
	return reader
}

// collectLockWaitTimeoutTotal reads the current value of
// protocluster_natskv_lock_wait_timeout_total from the reader and returns a
// map of outcome -> sum. Returns an empty map if the metric has not been
// recorded yet.
func collectLockWaitTimeoutTotal(t *testing.T, reader *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	result := make(map[string]int64)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "protocluster_natskv_lock_wait_timeout_total" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				for _, attr := range dp.Attributes.ToSlice() {
					if attr.Key == "outcome" {
						result[attr.Value.AsString()] += dp.Value
					}
				}
			}
		}
	}
	return result
}

// TestJanitorSkipsLiveRecords verifies that the janitor does not reap records
// that should be preserved:
//   - A young lock-only record (age < HardReapAge) is kept.
//   - A completed activation whose member key is present is kept.
func TestJanitorSkipsLiveRecords(t *testing.T) {
	p, _, il := setupPlacementTestCluster(t, "test-janitor-skip-live")
	p.isLeader.Store(true)

	ctx := context.Background()

	// Create a real member bucket so MemberKeyExists works.
	memberBucket, err := p.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: p.config.memberBucketName(il.cluster.Config.Name),
	})
	require.NoError(t, err)
	p.memberBucket = memberBucket

	memberID := "live-member-skip"

	// Write the member key so MemberKeyExists returns true.
	_, err = memberBucket.Put(ctx, p.memberKey(memberID), []byte(`{}`))
	require.NoError(t, err)

	t0 := time.Now()
	il.now = func() time.Time { return t0 }

	// Case 1: Young lock-only record (age = 0, HardReapAge = 60s).
	youngLockCI := testCI("TestKind", "skip-young-lock")
	youngLockKey := kvKey(youngLockCI)
	lockRec := activationRecord{LockID: "some-lock-id", MemberID: memberID}
	data, err := json.Marshal(&lockRec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, youngLockKey, data)
	require.NoError(t, err)

	// Case 2: Activation with a live member key.
	liveCI := testCI("TestKind", "skip-live-activation")
	liveKey := kvKey(liveCI)
	liveRec := activationRecord{
		PidID:      "TestKind/skip-live-activation",
		PidAddress: "host:8080",
		MemberID:   memberID,
	}
	data, err = json.Marshal(&liveRec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, liveKey, data)
	require.NoError(t, err)

	ac := &janitorAbsenceClock{}
	il.janitorSweep(ctx, ac)

	// Young lock must still be present.
	_, err = il.identities.Get(ctx, youngLockKey)
	assert.NoError(t, err, "young lock-only record (age < HardReapAge) must not be reaped")

	// Live activation must still be present.
	_, err = il.identities.Get(ctx, liveKey)
	assert.NoError(t, err, "activation with live member key must not be reaped")
}

// TestJanitorReapsAgedLocks verifies that a lock-only record older than
// HardReapAge is deleted by the janitor sweep.
func TestJanitorReapsAgedLocks(t *testing.T) {
	p, _, il := setupPlacementTestCluster(t, "test-janitor-reap-aged-lock")
	p.isLeader.Store(true)

	ctx := context.Background()

	t0 := time.Now()
	il.now = func() time.Time { return t0 }
	il.config.HardReapAge = 60 * time.Second

	ci := testCI("TestKind", "aged-lock-grain")
	key := kvKey(ci)

	// Write a lock-only record.
	rec := activationRecord{LockID: "stale-lock", MemberID: "gone-member"}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, key, data)
	require.NoError(t, err)

	// Sweep at t0: age = 0, record kept.
	ac := &janitorAbsenceClock{}
	il.janitorSweep(ctx, ac)
	_, err = il.identities.Get(ctx, key)
	assert.NoError(t, err, "lock-only record must be kept when age < HardReapAge")

	// Advance clock past HardReapAge.
	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	il.janitorSweep(ctx, ac)

	_, err = il.identities.Get(ctx, key)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "lock-only record must be reaped when age > HardReapAge")
}

// TestJanitorActivationAbsenceRule verifies the timestamp+count-gated
// activation cleanup rule:
//   - First observation (count=1): always kept regardless of time.
//   - Second observation (count=2) but age < ActivationAbsentGrace: kept.
//   - Third+ observation and age >= ActivationAbsentGrace: deleted.
//   - If member key is restored between sweeps, absence clock is cleared.
func TestJanitorActivationAbsenceRule(t *testing.T) {
	p, _, il := setupPlacementTestCluster(t, "test-janitor-absence-rule")
	p.isLeader.Store(true)

	ctx := context.Background()

	// Create the member bucket so MemberKeyExists can do real KV lookups.
	memberBucket, err := p.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: p.config.memberBucketName(il.cluster.Config.Name),
	})
	require.NoError(t, err)
	p.memberBucket = memberBucket

	t0 := time.Now()
	il.now = func() time.Time { return t0 }
	il.config.ActivationAbsentGrace = 60 * time.Second

	memberID := "test-member-absence-rule"
	ci := testCI("TestKind", "absence-grain")
	key := kvKey(ci)

	// Write a completed activation record. Member key is absent initially.
	rec := activationRecord{
		PidID:      "TestKind/absence-grain",
		PidAddress: "host:8080",
		MemberID:   memberID,
	}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, key, data)
	require.NoError(t, err)

	ac := &janitorAbsenceClock{}

	// Sweep 1 at t0+30s: first observation (count=1), kept regardless.
	il.now = func() time.Time { return t0.Add(30 * time.Second) }
	il.janitorSweep(ctx, ac)
	_, err = il.identities.Get(ctx, key)
	assert.NoError(t, err, "sweep 1 (count=1): record must be kept")

	// Sweep 2 at t0+45s: second observation (count=2), age=45s < 60s grace: kept.
	il.now = func() time.Time { return t0.Add(45 * time.Second) }
	il.janitorSweep(ctx, ac)
	_, err = il.identities.Get(ctx, key)
	assert.NoError(t, err, "sweep 2 (count=2, age<grace): record must be kept")

	// Sweep 3 at t0+91s: third observation (count=3), elapsed since firstSeen
	// (t0+30s) = 61s >= 60s grace: deleted.
	il.now = func() time.Time { return t0.Add(91 * time.Second) }
	il.janitorSweep(ctx, ac)
	_, err = il.identities.Get(ctx, key)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "sweep 3 (count>=2, age>=grace): record must be deleted")

	// Sub-test: member key restored between sweeps clears absence clock.
	ci2 := testCI("TestKind", "absence-grain-restored")
	key2 := kvKey(ci2)
	memberID2 := "test-member-restored"

	rec2 := activationRecord{
		PidID:      "TestKind/absence-grain-restored",
		PidAddress: "host:8080",
		MemberID:   memberID2,
	}
	data2, err := json.Marshal(&rec2)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, key2, data2)
	require.NoError(t, err)

	ac2 := &janitorAbsenceClock{}
	il.now = func() time.Time { return t0.Add(30 * time.Second) }

	// Sweep 1: member absent, first observation recorded.
	il.janitorSweep(ctx, ac2)
	_, err = il.identities.Get(ctx, key2)
	assert.NoError(t, err, "sweep 1 for key2 (count=1): record must be kept")

	// Restore the member key -- MemberKeyExists returns true now.
	_, err = memberBucket.Put(ctx, p.memberKey(memberID2), []byte(`{}`))
	require.NoError(t, err)

	// Sweep 2: member present, absence cleared.
	il.now = func() time.Time { return t0.Add(45 * time.Second) }
	il.janitorSweep(ctx, ac2)
	_, err = il.identities.Get(ctx, key2)
	assert.NoError(t, err, "after member restored: record must still be present")

	// Remove member key again.
	err = memberBucket.Delete(ctx, p.memberKey(memberID2))
	require.NoError(t, err)

	// Sweep 3: member absent again. Absence clock should have been cleared by
	// sweep 2, so this is only the first observation of the new absence window.
	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	il.janitorSweep(ctx, ac2)
	_, err = il.identities.Get(ctx, key2)
	assert.NoError(t, err, "after reset, sweep 3 is only first observation of new absence: record must be kept")
}

// TestWaitTimeoutMetricOutcomes verifies that recordWaitTimeoutOutcome
// increments the correct counter label based on what the follow-up KV Get finds:
//   - "activated_late": the key has a completed activation (PidID set).
//   - "still_locked": the key is absent or still a lock-only record.
//
// We test recordWaitTimeoutOutcome directly rather than through waitForActivation
// because the "activated_late" scenario requires the upgrade to be present at the
// moment of the follow-up Get, which races with waitForActivation's timeout when
// tested end-to-end. Direct invocation removes the race without losing coverage
// of the classification logic.
func TestWaitTimeoutMetricOutcomes(t *testing.T) {
	buildILWithMetrics := func(t *testing.T, suffix string) (*sdkmetric.ManualReader, *IdentityLookup) {
		t.Helper()
		reader := setupTestMeterProvider(t)

		srv := startEmbeddedNATS(t)
		_, js := connectNATS(t, srv)

		ctx := context.Background()
		identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket: "test_wait_timeout_" + suffix,
		})
		require.NoError(t, err)

		p, err := NewFromJetStream(js)
		require.NoError(t, err)
		p.providerMetrics = natskvmetrics.NewNatsKVMetrics(nil)
		p.metricsEnabled = true

		il := &IdentityLookup{
			provider:   p,
			identities: identities,
			config:     newDefaultConfig(),
			semaphore:  make(chan struct{}, 200),
			now:        time.Now,
		}
		return reader, il
	}

	t.Run("activated_late", func(t *testing.T) {
		reader, il := buildILWithMetrics(t, "late")
		ctx := context.Background()

		ci := testCI("TestKind", "waiter-late-grain")
		key := kvKey(ci)

		// Write a completed activation record so the follow-up Get finds PidID set.
		completedRec := activationRecord{
			PidID:      "TestKind/waiter-late-grain",
			PidAddress: "host:8080",
			MemberID:   "some-member",
		}
		data, err := json.Marshal(&completedRec)
		require.NoError(t, err)
		_, err = il.identities.Put(ctx, key, data)
		require.NoError(t, err)

		// Simulate the timeout path: recordWaitTimeoutOutcome does a Get and
		// should classify this as "activated_late".
		il.recordWaitTimeoutOutcome(ctx, key)

		counts := collectLockWaitTimeoutTotal(t, reader)
		assert.Equal(t, int64(1), counts["activated_late"],
			"activated_late counter must be incremented once")
		assert.Equal(t, int64(0), counts["still_locked"],
			"still_locked counter must not be incremented")
	})

	t.Run("still_locked", func(t *testing.T) {
		reader, il := buildILWithMetrics(t, "still")
		ctx := context.Background()

		ci := testCI("TestKind", "waiter-still-locked-grain")
		key := kvKey(ci)

		// Write a lock-only record (no PidID).
		lockRec := activationRecord{LockID: "held-lock", MemberID: "slow-member"}
		data, err := json.Marshal(&lockRec)
		require.NoError(t, err)
		_, err = il.identities.Put(ctx, key, data)
		require.NoError(t, err)

		// Simulate the timeout path: Get finds lock-only -> "still_locked".
		il.recordWaitTimeoutOutcome(ctx, key)

		counts := collectLockWaitTimeoutTotal(t, reader)
		assert.Equal(t, int64(0), counts["activated_late"],
			"activated_late counter must not be incremented")
		assert.Equal(t, int64(1), counts["still_locked"],
			"still_locked counter must be incremented once")
	})

	t.Run("key_absent", func(t *testing.T) {
		// A key that is absent (Get fails) is also classified as "still_locked".
		reader, il := buildILWithMetrics(t, "absent")
		ctx := context.Background()

		il.recordWaitTimeoutOutcome(ctx, "nonexistent/key")

		counts := collectLockWaitTimeoutTotal(t, reader)
		assert.Equal(t, int64(0), counts["activated_late"],
			"activated_late counter must not be incremented for absent key")
		assert.Equal(t, int64(1), counts["still_locked"],
			"still_locked counter must be incremented for absent key")
	})
}
