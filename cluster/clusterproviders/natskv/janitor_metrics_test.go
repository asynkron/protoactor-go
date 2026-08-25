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

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	natskvmetrics "github.com/awevoke/protoactor-go/cluster/clusterproviders/natskv/metrics"
	"github.com/awevoke/protoactor-go/remote"
)

// The janitor's cost was only ever visible as an aggregate NATS egress figure:
// one enumeration of the identities bucket, one Get per live key and one
// enumeration of the members bucket, all on the leader, once per
// JanitorInterval. Tasks 16 and 17 made that cheap; these tests pin that it is
// also VISIBLE, so a regression is a query rather than an inference.

// collectHistogram returns the single data point of the named float64
// histogram: its observation count, its sum, and the bucket boundaries the
// instrument declared. It fails the test when the instrument has no data.
func collectHistogram(t *testing.T, reader *sdkmetric.ManualReader, name string) (uint64, float64, []float64) {
	t.Helper()

	var rm metricdata.ResourceMetrics

	require.NoError(t, reader.Collect(context.Background(), &rm))

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}

			hist, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok, "%s is not a float64 histogram", name)
			require.Len(t, hist.DataPoints, 1, "%s must carry exactly one data point", name)

			dp := hist.DataPoints[0]

			return dp.Count, dp.Sum, dp.Bounds
		}
	}

	t.Fatalf("histogram %s was never recorded", name)

	return 0, 0, nil
}

// collectGauge returns the value of the named attribute-free int64 gauge, and
// whether it was recorded at all.
func collectGauge(t *testing.T, reader *sdkmetric.ManualReader, name string) (int64, bool) {
	t.Helper()

	var rm metricdata.ResourceMetrics

	require.NoError(t, reader.Collect(context.Background(), &rm))

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}

			g, ok := m.Data.(metricdata.Gauge[int64])
			require.True(t, ok, "%s is not an int64 gauge", name)
			require.Len(t, g.DataPoints, 1, "%s must carry exactly one data point", name)

			return g.DataPoints[0].Value, true
		}
	}

	return 0, false
}

// collectGaugeByLabel returns the named int64 gauge's value per value of the
// given attribute key.
func collectGaugeByLabel(t *testing.T, reader *sdkmetric.ManualReader, name, label string) map[string]int64 {
	t.Helper()

	var rm metricdata.ResourceMetrics

	require.NoError(t, reader.Collect(context.Background(), &rm))

	out := make(map[string]int64)

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}

			g, ok := m.Data.(metricdata.Gauge[int64])
			require.True(t, ok, "%s is not an int64 gauge", name)

			for _, dp := range g.DataPoints {
				for _, attr := range dp.Attributes.ToSlice() {
					if string(attr.Key) == label {
						out[attr.Value.AsString()] = dp.Value
					}
				}
			}
		}
	}

	return out
}

// collectCounter returns the value of the named attribute-free int64 counter.
func collectCounter(t *testing.T, reader *sdkmetric.ManualReader, name string) int64 {
	t.Helper()

	var rm metricdata.ResourceMetrics

	require.NoError(t, reader.Collect(context.Background(), &rm))

	total := int64(0)

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}

			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok, "%s is not an int64 sum", name)

			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
		}
	}

	return total
}

// steppedClock returns a clock whose FIRST call reports base and whose every
// later call reports base+step, so one janitorSweep (which reads the clock
// exactly twice on an empty bucket: once for start, once for the summary)
// measures exactly step.
func steppedClock(base time.Time, step time.Duration) func() time.Time {
	calls := 0

	return func() time.Time {
		calls++
		if calls == 1 {
			return base
		}

		return base.Add(step)
	}
}

// TestJanitorSweep_RecordsDuration pins that a sweep reports its own wall cost
// as a seconds histogram, at the boundaries the instrument declares. The
// boundaries are asserted here, not only the observation: the OTel SDK's
// DEFAULT boundaries are millisecond-scaled (0, 5, 10, ... 10000), so a
// seconds-valued histogram that forgot to declare its own would put every real
// sweep in one bucket and make every quantile -- including the one
// NatsKVJanitorSweepSlow evaluates -- meaningless.
func TestJanitorSweep_RecordsDuration(t *testing.T) {
	il, reader, _ := buildJanitorILWithMetrics(t, "test-janitor-duration")
	il.now = steppedClock(time.Now(), 300*time.Millisecond)

	il.janitorSweep(context.Background(), &janitorAbsenceClock{})

	count, sum, bounds := collectHistogram(t, reader, "protocluster_natskv_janitor_sweep_duration")
	assert.Equal(t, uint64(1), count, "one sweep must record exactly one observation")
	assert.InDelta(t, 0.3, sum, 1e-9, "the observation must be the sweep's wall time IN SECONDS")
	assert.Equal(t,
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		bounds,
		"the sweep histogram must declare seconds-scaled boundaries straddling the 250ms alert threshold")
}

// TestJanitorSweep_RecordsLiveKeys pins that the sweep publishes the LIVE key
// count it already enumerated, and that it is a gauge (last value) rather than
// a counter: the second sweep must report the smaller set, not the sum.
func TestJanitorSweep_RecordsLiveKeys(t *testing.T) {
	il, reader, _ := buildJanitorILWithMetrics(t, "test-janitor-live-keys")
	ctx := context.Background()

	rec, err := json.Marshal(&activationRecord{PidID: "p", MemberID: il.memberID})
	require.NoError(t, err)

	keys := []string{
		kvKey(testCI("TestKind", "live-1")),
		kvKey(testCI("TestKind", "live-2")),
		kvKey(testCI("TestKind", "live-3")),
	}
	for _, k := range keys {
		_, putErr := il.identities.Put(ctx, k, rec)
		require.NoError(t, putErr)
	}

	il.janitorSweep(ctx, &janitorAbsenceClock{})

	live, ok := collectGauge(t, reader, "protocluster_natskv_janitor_live_keys")
	require.True(t, ok, "a completed sweep must report its live-key count")
	assert.Equal(t, int64(3), live)

	require.NoError(t, il.identities.Purge(ctx, keys[0]))
	il.janitorSweep(ctx, &janitorAbsenceClock{})

	live, ok = collectGauge(t, reader, "protocluster_natskv_janitor_live_keys")
	require.True(t, ok)
	assert.Equal(t, int64(2), live, "live keys is a gauge: the second sweep reports the current set, not the sum")
}

// TestJanitorSweep_ListFailureRecordsNoCost pins the one path that must NOT
// record: a sweep whose ListKeys failed enumerated nothing, so a duration
// sample would report a suspiciously fast sweep and a live-key gauge of zero
// would claim the bucket is empty. recordJanitorSweep("error") is that path's
// signal.
func TestJanitorSweep_ListFailureRecordsNoCost(t *testing.T) {
	il, reader, _ := buildJanitorILWithMetrics(t, "test-janitor-list-failure")

	// A cancelled context makes ListKeys fail before anything is enumerated.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	il.janitorSweep(ctx, &janitorAbsenceClock{})

	_, ok := collectGauge(t, reader, "protocluster_natskv_janitor_live_keys")
	assert.False(t, ok, "a sweep that could not enumerate must not claim a live-key count")

	sweeps := collectCounterByLabel(t, reader, "protocluster_natskv_janitor_sweep_total", "outcome")
	assert.Equal(t, int64(1), sweeps["error"], "the failed sweep is still counted as an error sweep")
}

// TestCasDelete_PurgeOutcomes pins the three shapes of casDelete's returned
// error that its two production recorders (janitor.go's hard-reap-lock and
// absent-member sites, via recordTombstonePurge) key their tombstone-purge
// counting on. The counting itself moved OUT of casDelete (I1, the janitor-
// scoping fix): this is now casDelete's own return-value contract, pinned
// independent of who calls it or what they do with the answer.
func TestCasDelete_PurgeOutcomes(t *testing.T) {
	il, _ := newIdentityLookupForTest(t)

	ctx := context.Background()
	key := kvKey(testCI("TestKind", "purge-me"))

	rec, err := json.Marshal(&activationRecord{PidID: "p", MemberID: il.memberID})
	require.NoError(t, err)
	rev, err := il.identities.Put(ctx, key, rec)
	require.NoError(t, err)

	assert.NoError(t, il.casDelete(ctx, key, rev, "test"),
		"a purge that applies must return nil")

	// A CAS conflict: the key this call still names by its now-stale
	// revision was already purged by the call above.
	assert.Error(t, il.casDelete(ctx, key, rev, "test"),
		"a CAS-rejected purge (the key is already gone at that revision) must return a non-nil error")

	// The quirk D5 documents: a purge of a key that never existed, guarded at
	// revision 0 ("this subject holds nothing"), is ACCEPTED by nats-server
	// and DOES store a marker -- err is nil even though nothing was "removed".
	// Unreachable from production: casDelete's two janitor sites both pass a
	// revision read from a live entry.
	assert.NoError(t, il.casDelete(ctx, kvKey(testCI("TestKind", "never-existed")), 0, "test"),
		"a purge accepted at revision 0 must return nil -- it stores a marker despite deleting nothing")
}

// TestJanitorReap_RecordsTombstonePurge pins the POSITIVE half of I1: the
// janitor's own reap sites (janitor.go's "janitor/hard-reap-lock" and
// "janitor/absent-member") must still increment the tombstone-purge counter
// now that the recording lives at their call sites instead of inside
// casDelete. Drives a real janitorSweep hard-reap-lock pass rather than
// calling casDelete directly, so the wiring at the actual call site is what
// is pinned, not just the recorder function in isolation.
func TestJanitorReap_RecordsTombstonePurge(t *testing.T) {
	il, env := newIdentityLookupForTest(t, withMemberBucket())
	reader := setupTestMeterProvider(t)
	env.provider.providerMetrics = natskvmetrics.NewNatsKVMetrics(nil)
	env.provider.metricsEnabled = true

	ctx := context.Background()
	t0 := time.Now()
	il.now = func() time.Time { return t0 }
	il.config.HardReapAge = 60 * time.Second

	key := kvKey(testCI("TestKind", "janitor-reap-purge"))
	rec, err := json.Marshal(&activationRecord{LockID: "l", MemberID: "gone"})
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, key, rec)
	require.NoError(t, err)

	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	il.janitorSweep(ctx, &janitorAbsenceClock{})

	const metricName = "protocluster_natskv_janitor_tombstone_purge_total"
	assert.Equal(t, int64(1), collectCounter(t, reader, metricName),
		"the janitor's own hard-reap-lock purge must be counted")

	_, err = il.identities.Get(ctx, key)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "the aged lock must actually be reaped")
}

// TestCasDelete_NonJanitorSite_DoesNotRecordTombstonePurge pins the NEGATIVE
// half of I1, and is the fix wave's red-first test: against the pre-fix code
// (the counter incremented inside casDelete itself, unconditionally on every
// one of its fifteen production call sites) this test failed, because
// removeMemberID's casDelete call -- site "removeMemberID", not one of the
// janitor's two sites -- counted too.
// protocluster_natskv_janitor_tombstone_purge_total's name, HELP text and the
// runbook's NatsKVJanitorSweepSlow "First checks" step all promise a
// JANITOR-scoped count; the other thirteen call sites -- resolveIdentity's
// grace/no-target paths, activateLocal/activateRemote's store-failure
// cleanups, cleanupOwnRecord, removeMemberID -- run on ordinary CAS losses,
// lock steals and member departures and must not move it.
//
// Drives the real removeMemberID production path (topology leave / graceful
// shutdown) rather than casDelete directly with a fabricated site string, the
// same end-to-end style TestStartMember_RecordsMarkerTTLGauge uses for the
// marker gauge: the WIRING is pinned, not just the recorder.
func TestCasDelete_NonJanitorSite_DoesNotRecordTombstonePurge(t *testing.T) {
	il, env := newIdentityLookupForTest(t, withMemberBucket())
	reader := setupTestMeterProvider(t)
	env.provider.providerMetrics = natskvmetrics.NewNatsKVMetrics(nil)
	env.provider.metricsEnabled = true

	ctx := context.Background()
	deadMember := "gone-member"
	ci := testCI("TestKind", "non-janitor-purge")

	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, rev, deadMember, "dead-host:8080", "TestKind/non-janitor-purge"))

	// removeMemberID CAS-deletes the activation it just stored, via
	// reapDepartedMemberIdentity's "removeMemberID"-sited casDelete call -- a
	// real production non-janitor site.
	il.removeMemberID(ctx, deadMember)

	rec := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "removeMemberID must still have deleted the activation")

	const metricName = "protocluster_natskv_janitor_tombstone_purge_total"
	assert.Equal(t, int64(0), collectCounter(t, reader, metricName),
		"a non-janitor casDelete must not be counted as a janitor tombstone purge")
}

// TestStartMember_RecordsMarkerTTLGauge pins the marker-accumulation signal.
// A bucket created without KV marker TTLs retains every delete marker forever,
// and today the ONLY evidence is one WARN at process start -- a log search, on
// a cluster that has already been accumulating for however long it has been
// up. The gauge makes "did this cluster get the fix" a query, per bucket:
// WithIdentityBucket can rotate the identities bucket, but the tracking
// bucket's name is derived and has no override, so which one is degraded
// changes the remedy.
//
// It is recorded from the PROVIDER's start, not from IdentityLookup.Setup,
// because Cluster.StartMember calls Setup BEFORE ClusterProvider.StartMember
// (cluster/cluster.go:159) -- at Setup time providerMetrics is still nil. This
// test therefore drives the real StartMember rather than the recorder alone.
func TestStartMember_RecordsMarkerTTLGauge(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	const clusterName = "markerttl"

	// The reader must belong to the provider the ACTOR SYSTEM installs, not to
	// one installed beside it: actor.NewActorSystem with a metrics provider
	// calls otel.SetMeterProvider itself (actor/config.go), so a provider set
	// up before it is replaced -- and NewNatsKVMetrics reads the global meter.
	// That is also how production wires it (lib/components/otelmetrics).
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prevMeterProvider := otel.GetMeterProvider()

	t.Cleanup(func() {
		_ = mp.Shutdown(context.Background())
		otel.SetMeterProvider(prevMeterProvider)
	})

	system := actor.NewActorSystem(actor.WithMetricProviders(mp))
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)
	require.NoError(t, c.Remote.Start())

	il := p.IdentityLookup()
	il.Setup(c, nil, false)
	require.NoError(t, p.StartMember(c))

	t.Cleanup(func() {
		_ = p.Shutdown(true)
		il.Shutdown()
		c.Remote.Shutdown(true)
	})

	byBucket := collectGaugeByLabel(t, reader, "protocluster_natskv_marker_ttl_enabled", "bucket")
	assert.Equal(t, map[string]int64{
		"protoactor_" + clusterName + "_identities":          1,
		"protoactor_" + clusterName + "_identities_tracking": 1,
	}, byBucket, "both identity buckets must report their marker-TTL state, one series each")
}

// TestStartClient_RecordsMarkerTTLGauge is TestStartMember_RecordsMarkerTTLGauge's
// other half: recordMarkerTTLEnabled is called from BOTH StartMember and
// StartClient (natskv_provider.go), because a client never runs StartMember
// but still owns identity buckets whose marker-TTL state is exactly as
// interesting on it as on a member. Drives the real StartClient rather than
// the recorder alone, so the wiring on the CLIENT path is pinned too, not
// just the member one.
func TestStartClient_RecordsMarkerTTLGauge(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	const clusterName = "markerttl-client"

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prevMeterProvider := otel.GetMeterProvider()

	t.Cleanup(func() {
		_ = mp.Shutdown(context.Background())
		otel.SetMeterProvider(prevMeterProvider)
	})

	system := actor.NewActorSystem(actor.WithMetricProviders(mp))
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)
	require.NoError(t, c.Remote.Start())

	il := p.IdentityLookup()
	il.Setup(c, nil, true)
	require.NoError(t, p.StartClient(c))

	t.Cleanup(func() {
		_ = p.Shutdown(true)
		il.Shutdown()
		c.Remote.Shutdown(true)
	})

	byBucket := collectGaugeByLabel(t, reader, "protocluster_natskv_marker_ttl_enabled", "bucket")
	assert.Equal(t, map[string]int64{
		"protoactor_" + clusterName + "_identities":          1,
		"protoactor_" + clusterName + "_identities_tracking": 1,
	}, byBucket, "a client's identity buckets must also report their marker-TTL state")
}

// TestSetup_DottedMemberID_RecordsNoMarkerTTLGauge pins the ABSENCE half of
// D8: a lookup whose Setup failed (the dotted-member-id guard) must record NO
// series at all for the marker gauge, not a 0. Setup's guard runs before
// either bucket is created (Setup returns before touching js.CreateOrUpdateKeyValue),
// so markerTTLStates is left empty and recordMarkerTTLEnabled's own
// `il.setupErr != nil` check is what keeps it that way -- a 0 here would
// misreport a marker-retention problem in place of what actually happened,
// which is that Setup never created any buckets to describe. p.StartMember
// itself does NOT check il.setupErr (Cluster.StartMember calls Setup before
// ClusterProvider.StartMember and never inspects the result), so it still
// succeeds and still creates the PROVIDER's own member/leader buckets --
// this test drives that real sequence rather than asserting on the recorder
// in isolation.
func TestSetup_DottedMemberID_RecordsNoMarkerTTLGauge(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	const clusterName = "markerttl-dotted"

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prevMeterProvider := otel.GetMeterProvider()

	t.Cleanup(func() {
		_ = mp.Shutdown(context.Background())
		otel.SetMeterProvider(prevMeterProvider)
	})

	// An FQDN system id is how a dotted member id arrives in a real
	// deployment (actorsystem passes actor.WithSystemID(hostname())); see
	// TestSetup_RejectsDottedMemberID in tracking_subkeys_test.go.
	system := actor.NewActorSystem(actor.WithMetricProviders(mp), actor.WithSystemID("node.host.example.com"))
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)
	require.NoError(t, c.Remote.Start())

	il := p.IdentityLookup()
	il.Setup(c, nil, false)
	require.Error(t, il.setupErr, "a dotted member id must fail Setup")

	require.NoError(t, p.StartMember(c), "StartMember does not consult il.setupErr and must still succeed")

	t.Cleanup(func() {
		_ = p.Shutdown(true)
		il.Shutdown()
		c.Remote.Shutdown(true)
	})

	byBucket := collectGaugeByLabel(t, reader, "protocluster_natskv_marker_ttl_enabled", "bucket")
	assert.Empty(t, byBucket, "a lookup whose Setup failed must record no marker-TTL series at all")
}
