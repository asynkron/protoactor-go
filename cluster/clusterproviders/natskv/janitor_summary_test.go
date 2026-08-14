package natskv

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	natskvmetrics "github.com/awevoke/protoactor-go/cluster/clusterproviders/natskv/metrics"
	"github.com/awevoke/protoactor-go/remote"
)

// collectCounterByLabel reads the named Int64 counter and returns a map of the
// given label's value -> summed count.
func collectCounterByLabel(t *testing.T, reader *sdkmetric.ManualReader, name, label string) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	result := make(map[string]int64)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				for _, attr := range dp.Attributes.ToSlice() {
					if string(attr.Key) == label {
						result[attr.Value.AsString()] += dp.Value
					}
				}
			}
		}
	}
	return result
}

// buildJanitorILWithMetrics builds a leader IdentityLookup with a real members
// bucket and metrics enabled, whose cluster logger is routed to a capturing
// handler so the janitor summary line can be asserted at the exact log level.
func buildJanitorILWithMetrics(t *testing.T, name string) (*IdentityLookup, *sdkmetric.ManualReader, *capturingHandler) {
	t.Helper()
	reader := setupTestMeterProvider(t)
	h := &capturingHandler{}

	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	kindProps := actor.PropsFromFunc(func(actor.Context) {})
	kind := cluster.NewKind("TestKind", kindProps)

	// Route the cluster logger through the capturing handler.
	system := actor.NewActorSystem(actor.WithLoggerFactory(func(*actor.ActorSystem) *slog.Logger {
		return slog.New(h)
	}))
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(name, p, p.IdentityLookup(), remoteConfig,
		cluster.WithKinds(kind))
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)
	require.NoError(t, c.Remote.Start())
	c.InitKindsForTest(kind)

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)

	host, port, err := c.ActorSystem.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{Host: host, Port: int32(port), Id: il.memberID, Kinds: []string{"TestKind"}}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	t.Cleanup(func() {
		il.Shutdown()
		c.Remote.Shutdown(true)
	})

	p.isLeader.Store(true)
	p.providerMetrics = natskvmetrics.NewNatsKVMetrics(nil)
	p.metricsEnabled = true

	memberBucket, err := p.js.CreateOrUpdateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: p.config.memberBucketName(name),
	})
	require.NoError(t, err)
	p.memberBucket = memberBucket

	// Reset the capture buffer so only the sweep under test is asserted (Setup
	// and topology emit their own log lines).
	h.mu.Lock()
	h.recs = nil
	h.mu.Unlock()

	return il, reader, h
}

// TestJanitorSweepSummaryQuiet verifies a sweep with no work logs at Debug and
// records a "clean" sweep outcome counter.
func TestJanitorSweepSummaryQuiet(t *testing.T) {
	il, reader, h := buildJanitorILWithMetrics(t, "test-janitor-summary-quiet")

	ctx := context.Background()
	ac := &janitorAbsenceClock{}
	il.janitorSweep(ctx, ac)

	assert.Equal(t, 1, h.countByLevelAndMsg(slog.LevelDebug, "sweep complete"),
		"quiet sweep must log summary at Debug")
	assert.Equal(t, 0, h.countByLevelAndMsg(slog.LevelInfo, "sweep complete"),
		"quiet sweep must not log summary at Info")

	sweeps := collectCounterByLabel(t, reader, "protocluster_natskv_janitor_sweep_total", "outcome")
	assert.Equal(t, int64(1), sweeps["clean"], "clean sweep counter must increment")
}

// TestJanitorSweepSummaryWork verifies a sweep that reaps a lock logs at Info
// and records "work" + a lock reap counter.
func TestJanitorSweepSummaryWork(t *testing.T) {
	il, reader, h := buildJanitorILWithMetrics(t, "test-janitor-summary-work")

	ctx := context.Background()
	t0 := time.Now()
	il.now = func() time.Time { return t0 }
	il.config.HardReapAge = 60 * time.Second

	// Seed an aged lock-only record.
	ci := testCI("TestKind", "summary-aged-lock")
	key := kvKey(ci)
	data, err := json.Marshal(&activationRecord{LockID: "l", MemberID: "gone"})
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, key, data)
	require.NoError(t, err)

	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	ac := &janitorAbsenceClock{}
	il.janitorSweep(ctx, ac)

	assert.Equal(t, 1, h.countByLevelAndMsg(slog.LevelInfo, "sweep complete"),
		"working sweep must log summary at Info")

	sweeps := collectCounterByLabel(t, reader, "protocluster_natskv_janitor_sweep_total", "outcome")
	assert.Equal(t, int64(1), sweeps["work"], "work sweep counter must increment")
	reaps := collectCounterByLabel(t, reader, "protocluster_natskv_janitor_reap_total", "type")
	assert.Equal(t, int64(1), reaps["lock"], "lock reap counter must increment")

	_, err = il.identities.Get(ctx, key)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "aged lock must be reaped")
}
