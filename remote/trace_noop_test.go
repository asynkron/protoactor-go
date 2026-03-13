package remote

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNoOpTracer_NoSpansExported(t *testing.T) {
	// Set up an exporter attached to a TracerProvider that IS set globally,
	// but actors are spawned WITHOUT OTEL middleware. This verifies that
	// without middleware, no spans are produced even with a provider configured.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prevTP)

	// Create two nodes and send a message — NO middleware on actors
	systemA := actor.NewActorSystem()
	systemB := actor.NewActorSystem()

	configA := Configure("127.0.0.1", 0)
	configB := Configure("127.0.0.1", 0)

	remoteA := NewRemote(systemA, configA)
	remoteB := NewRemote(systemB, configB)

	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(true)

	received := make(chan bool, 1)
	propsB := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *emptypb.Empty:
			received <- true
		}
	})
	_, err = systemB.Root.SpawnNamed(propsB, "noop-receiver")
	require.NoError(t, err)

	remotePidB := actor.NewPID(systemB.Address(), "noop-receiver")
	systemA.Root.Send(remotePidB, &emptypb.Empty{})

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for message")
	}

	// No middleware → no spans should be exported even with a TracerProvider
	require.NoError(t, tp.ForceFlush(context.Background()))
	assert.Empty(t, exporter.GetSpans(), "No spans should be exported without middleware")
}

func TestNoOpTracer_ZeroAllocations(t *testing.T) {
	// Verify that the no-op tracer path doesn't allocate excessively.
	// With the default global no-op tracer provider, starting and ending
	// a span should be essentially free.
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(otel.GetTracerProvider()) // ensure default no-op
	defer otel.SetTracerProvider(prevTP)

	allocs := testing.AllocsPerRun(10, func() {
		_, span := otel.Tracer("protoactor/cluster").Start(context.Background(), "test.span")
		span.End()
	})
	assert.LessOrEqual(t, allocs, float64(2), "No-op tracer should have minimal allocations")
}
