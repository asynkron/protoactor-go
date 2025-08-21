package remote_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	remote "github.com/asynkron/protoactor-go/remote"
	"github.com/asynkron/protoactor-go/testkit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func newSystem(provider *sdkmetric.MeterProvider) *actor.ActorSystem {
	cfg := actor.NewConfig()
	cfg.MetricsProvider = provider
	cfg.MetricsEnabled = true
	cfg.LoggerFactory = func(_ *actor.ActorSystem) *slog.Logger {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return actor.NewActorSystemWithConfig(cfg)
}

func TestRemoteMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	system1 := newSystem(provider)
	remote1 := remote.NewRemote(system1, remote.Configure("127.0.0.1", 0))
	remote1.Start()
	defer system1.Shutdown()

	system2 := newSystem(provider)
	remote2 := remote.NewRemote(system2, remote.Configure("127.0.0.1", 0))
	// collect mailbox stats to determine when the request is handled
	stats := testkit.NewTestMailboxStats(func(m interface{}) bool {
		_, ok := m.(*remote.ActorPidRequest)
		return ok
	})
	remote2.Register("echo", actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *remote.ActorPidRequest:
			// no-op
			return
		}
	}, testkit.WithReceiveStats(stats)))
	remote2.Start()
	defer system2.Shutdown()

	resp, err := remote1.Spawn(system2.Address(), "echo", time.Second)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if resp.Pid == nil {
		t.Fatalf("expected pid from spawn")
	}

	system1.Root.Send(resp.Pid, &remote.ActorPidRequest{})
	<-stats.Reset // wait for the remote actor to handle the request

	remote1.Shutdown(true)
	remote2.Shutdown(true)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	assertSumMetric(t, rm, "protoremote_endpoint_connected_count", map[string]string{
		"id":                 system1.ID,
		"address":            system1.Address(),
		"destinationaddress": system2.Address(),
	})
	assertSumMetric(t, rm, "protoremote_endpoint_disconnected_count", map[string]string{
		"id":      system1.ID,
		"address": system1.Address(),
	})
	assertSumMetric(t, rm, "protoremote_message_serialize_count", map[string]string{
		"id":          system1.ID,
		"address":     system1.Address(),
		"messagetype": "remote.ActorPidRequest",
	})
	assertSumMetric(t, rm, "protoremote_message_deserialize_count", map[string]string{
		"id":          system2.ID,
		"address":     system2.Address(),
		"messagetype": "remote.ActorPidRequest",
	})
	assertSumMetric(t, rm, "protoremote_spawn_count", map[string]string{
		"id":      system2.ID,
		"address": system2.Address(),
		"kind":    "echo",
	})
	assertHistogramMetric(t, rm, "protoremote_write_duration", map[string]string{
		"id":                 system1.ID,
		"address":            system1.Address(),
		"destinationaddress": system2.Address(),
	})
}

func hasAttributes(set attribute.Set, attrs map[string]string) bool {
	for k, v := range attrs {
		if val, ok := set.Value(attribute.Key(k)); !ok || val.AsString() != v {
			return false
		}
	}
	return true
}

func assertSumMetric(t *testing.T, rm metricdata.ResourceMetrics, name string, attrs map[string]string) {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if data, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, dp := range data.DataPoints {
					if hasAttributes(dp.Attributes, attrs) && dp.Value > 0 {
						return
					}
				}
			}
		}
	}
	t.Fatalf("metric %s with attrs %v not found", name, attrs)
}

func assertHistogramMetric(t *testing.T, rm metricdata.ResourceMetrics, name string, attrs map[string]string) {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if data, ok := m.Data.(metricdata.Histogram[float64]); ok {
				for _, dp := range data.DataPoints {
					if hasAttributes(dp.Attributes, attrs) && dp.Count > 0 {
						return
					}
				}
			}
		}
	}
	t.Fatalf("metric %s with attrs %v not found", name, attrs)
}
