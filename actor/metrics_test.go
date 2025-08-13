package actor

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestActorMetrics ensures core actor metrics record when enabled.
func TestActorMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	cfg := NewConfig()
	cfg.MetricsProvider = provider
	cfg.MetricsEnabled = true
	cfg.LoggerFactory = func(_ *ActorSystem) *slog.Logger {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	system := NewActorSystemWithConfig(cfg)
	defer system.Shutdown()

	var wg sync.WaitGroup
	wg.Add(2)
	pid := system.Root.Spawn(PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			wg.Done()
		}
	}))

	system.Root.Send(pid, "a")
	system.Root.Send(pid, "b")
	wg.Wait()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	foundSpawn, foundMailbox, foundDuration := false, false, false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "protoactor_actor_spawn_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 && data.DataPoints[0].Value > 0 {
					foundSpawn = true
				}
			case "protoactor_actor_mailbox_length":
				if data, ok := m.Data.(metricdata.Histogram[int64]); ok && len(data.DataPoints) > 0 {
					foundMailbox = true
				}
			case "protoactor_actor_messagereceive_duration":
				if data, ok := m.Data.(metricdata.Histogram[float64]); ok && len(data.DataPoints) > 0 {
					foundDuration = true
				}
			}
		}
	}

	if !foundSpawn || !foundMailbox || !foundDuration {
		t.Fatalf("missing metrics spawn:%v mailbox:%v duration:%v", foundSpawn, foundMailbox, foundDuration)
	}
}

// TestActorLifecycleMetrics verifies metrics related to actor failures, restarts
// and stops are emitted.
func TestActorLifecycleMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	cfg := NewConfig()
	cfg.MetricsProvider = provider
	cfg.MetricsEnabled = true
	cfg.LoggerFactory = func(_ *ActorSystem) *slog.Logger {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	system := NewActorSystemWithConfig(cfg)
	defer system.Shutdown()

	var wg sync.WaitGroup
	wg.Add(1)
	pid := system.Root.Spawn(PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case string:
			// trigger a failure which should restart the actor
			panic("boom")
		case *Restarting:
			ctx.Stop(ctx.Self())
			wg.Done()
		}
	}))

	system.Root.Send(pid, "fail")
	wg.Wait()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	foundFailure, foundRestart, foundStop := false, false, false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "protoactor_actor_failure_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 && data.DataPoints[0].Value > 0 {
					foundFailure = true
				}
			case "protoactor_actor_restarted_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 && data.DataPoints[0].Value > 0 {
					foundRestart = true
				}
			case "protoactor_actor_stopped_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 && data.DataPoints[0].Value > 0 {
					foundStop = true
				}
			}
		}
	}

	if !foundFailure || !foundRestart || !foundStop {
		t.Fatalf("missing metrics failure:%v restart:%v stop:%v", foundFailure, foundRestart, foundStop)
	}
}

// TestDeadLetterMetrics ensures dead letter messages increment the metric.
func TestDeadLetterMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	cfg := NewConfig()
	cfg.MetricsProvider = provider
	cfg.MetricsEnabled = true
	cfg.LoggerFactory = func(_ *ActorSystem) *slog.Logger {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	system := NewActorSystemWithConfig(cfg)
	defer system.Shutdown()

	ch := make(chan struct{}, 1)
	sub := system.EventStream.Subscribe(func(evt interface{}) {
		if _, ok := evt.(*DeadLetterEvent); ok {
			ch <- struct{}{}
		}
	})
	defer system.EventStream.Unsubscribe(sub)

	pid := NewPID(system.Address(), "unknown")
	system.Root.Send(pid, "msg")
	<-ch

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "protoactor_deadletter_count" {
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 && data.DataPoints[0].Value > 0 {
					found = true
				}
			}
		}
	}

	if !found {
		t.Fatalf("missing dead letter metric")
	}
}

// TestNewMetricsSetsGlobalProvider verifies that NewMetrics installs the supplied
// meter provider as the process-wide default. Because this mutates global
// state, the previous provider is restored after the test completes.
func TestNewMetricsSetsGlobalProvider(t *testing.T) {
	// Save and restore the existing provider so the global state does not
	// leak to other tests.
	prev := otel.GetMeterProvider()
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	cfg := NewConfig()
	cfg.MetricsEnabled = true
	system := NewActorSystemWithConfig(cfg)
	defer system.Shutdown()

	provider := sdkmetric.NewMeterProvider()
	NewMetrics(system, provider)

	if got := otel.GetMeterProvider(); got != provider {
		t.Fatalf("expected global meter provider to change")
	}
}
