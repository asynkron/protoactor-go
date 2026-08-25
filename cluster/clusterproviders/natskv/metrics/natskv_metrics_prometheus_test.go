package natskvmetrics_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	natskvmetrics "github.com/awevoke/protoactor-go/cluster/clusterproviders/natskv/metrics"
)

// The OTel instrument name is not the series name an alert rule has to spell.
// The Prometheus bridge (lib/components/otelmetrics in the consuming platform)
// appends the unit for a seconds instrument and _total for a counter, both
// only when the name does not already carry them, so the mapping is neither
// identity nor mechanical.
//
// deploy/base/manifests/alert-rules/lgp-nats.yaml, its promtool fixture and
// docs/runbooks/alerts.md all carry these strings literally; nothing else in
// this repository would notice if a rename made all three fictional, and an
// alert over a metric that does not exist is silently permanently quiet. This
// test is the pin that makes such a rename fail here instead.
func TestExportedPrometheusNames(t *testing.T) {
	reg := prometheus.NewRegistry()

	exp, err := otelprom.New(
		otelprom.WithRegisterer(reg),
		otelprom.WithoutScopeInfo(),
		otelprom.WithoutTargetInfo(),
	)
	require.NoError(t, err)

	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exp))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)

	t.Cleanup(func() {
		_ = mp.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})

	m := natskvmetrics.NewNatsKVMetrics(nil)

	ctx := context.Background()
	m.JanitorSweepDuration.Record(ctx, 0.3)
	m.JanitorLiveKeys.Record(ctx, 7)
	m.JanitorTombstonePurgeTotal.Add(ctx, 1)
	m.MarkerTTLEnabled.Record(ctx, 1)

	families, err := reg.Gather()
	require.NoError(t, err)

	seen := make(map[string]bool, len(families))
	for _, f := range families {
		seen[f.GetName()] = true
	}

	for _, want := range []string{
		"protocluster_natskv_janitor_sweep_duration_seconds",
		"protocluster_natskv_janitor_live_keys",
		"protocluster_natskv_janitor_tombstone_purge_total",
		"protocluster_natskv_marker_ttl_enabled",
	} {
		assert.True(t, seen[want], "alert rules and the runbook spell %q; the bridge exported %v", want, seen)
	}

	// The alert evaluates
	// histogram_quantile(0.99, ..._sweep_duration_seconds_bucket) > 0.25, so
	// the le=0.25 boundary must exist and the scale must be seconds. The SDK's
	// default boundaries are millisecond-scaled and carry no 0.25 at all.
	var bounds []float64

	for _, f := range families {
		if f.GetName() != "protocluster_natskv_janitor_sweep_duration_seconds" {
			continue
		}

		require.Len(t, f.GetMetric(), 1)

		for _, b := range f.GetMetric()[0].GetHistogram().GetBucket() {
			bounds = append(bounds, b.GetUpperBound())
		}
	}

	assert.Equal(t,
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		bounds,
		"the exported le values are the ones the promtool fixture must use")
}
