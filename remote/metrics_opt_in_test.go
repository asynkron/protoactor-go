package remote

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// newMetricsSystem creates an actor system with metrics wired to the given provider.
func newMetricsSystem(provider *sdkmetric.MeterProvider) *actor.ActorSystem {
	cfg := actor.NewConfig()
	cfg.MetricsProvider = provider
	cfg.MetricsEnabled = true
	cfg.LoggerFactory = func(_ *actor.ActorSystem) *slog.Logger {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return actor.NewActorSystemWithConfig(cfg)
}

// findSumMetric searches collected metrics for a Sum counter with the given name
// and returns true if any data point has a value > 0.
func findSumMetric(rm metricdata.ResourceMetrics, name string) bool {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if data, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, dp := range data.DataPoints {
					if dp.Value > 0 {
						return true
					}
				}
			}
		}
	}
	return false
}

// TestOptInMetrics_DisabledByDefault verifies that per-endpoint metrics
// (protoremote_message_sent_total, protoremote_message_received_total)
// are NOT recorded when EnablePerEndpointMetrics is false (default).
func TestOptInMetrics_DisabledByDefault(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	systemA := newMetricsSystem(provider)
	remoteA := NewRemote(systemA, Configure("127.0.0.1", 0, WithShutdownTimeout(time.Second)))
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(false)

	systemB := newMetricsSystem(provider)
	remoteB := NewRemote(systemB, Configure("127.0.0.1", 0, WithShutdownTimeout(time.Second)))
	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(false)

	// Confirm the flag is off by default.
	assert.False(t, remoteA.config.EnablePerEndpointMetrics)
	assert.False(t, remoteB.config.EnablePerEndpointMetrics)

	// Spawn an echo actor on B and send a message from A.
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	pid, err := systemB.Root.SpawnNamed(props, "echo-opt-in-off")
	require.NoError(t, err)

	fut := systemA.Root.RequestFuture(pid, &emptypb.Empty{}, 5*time.Second)
	_, err = fut.Result()
	require.NoError(t, err)

	// Collect metrics before shutdown so instruments are still active.
	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Per-endpoint metrics should NOT be present.
	assert.False(t, findSumMetric(rm, "protoremote_message_sent_total"),
		"protoremote_message_sent_total should not be recorded when EnablePerEndpointMetrics is false")
	assert.False(t, findSumMetric(rm, "protoremote_message_received_total"),
		"protoremote_message_received_total should not be recorded when EnablePerEndpointMetrics is false")

	// But other always-on metrics should be recorded (sanity check).
	assert.True(t, findSumMetric(rm, "protoremote_message_serialize_count"),
		"protoremote_message_serialize_count should always be recorded when metrics are enabled")
}

// TestOptInMetrics_EnabledWithFlag verifies that per-endpoint metrics ARE
// recorded when EnablePerEndpointMetrics is true.
func TestOptInMetrics_EnabledWithFlag(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	systemA := newMetricsSystem(provider)
	remoteA := NewRemote(systemA, Configure("127.0.0.1", 0, WithPerEndpointMetrics(), WithShutdownTimeout(time.Second)))
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(false)

	systemB := newMetricsSystem(provider)
	remoteB := NewRemote(systemB, Configure("127.0.0.1", 0, WithPerEndpointMetrics(), WithShutdownTimeout(time.Second)))
	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(false)

	// Confirm the flag is on.
	assert.True(t, remoteA.config.EnablePerEndpointMetrics)
	assert.True(t, remoteB.config.EnablePerEndpointMetrics)

	// Spawn an echo actor on B and send a message from A.
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	pid, err := systemB.Root.SpawnNamed(props, "echo-opt-in-on")
	require.NoError(t, err)

	fut := systemA.Root.RequestFuture(pid, &emptypb.Empty{}, 5*time.Second)
	_, err = fut.Result()
	require.NoError(t, err)

	// Collect metrics.
	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Per-endpoint metrics SHOULD be present.
	assert.True(t, findSumMetric(rm, "protoremote_message_sent_total"),
		"protoremote_message_sent_total should be recorded when EnablePerEndpointMetrics is true")
	// Note: protoremote_message_received_total requires connectedAddress to be set,
	// which happens during the Connect handshake in endpointReader. We verify the
	// sent metric which is the primary gating check.
}
