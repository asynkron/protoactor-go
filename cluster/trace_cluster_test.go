package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestClusterOperationSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	}()

	// Create a test kind that echoes back
	testKindProps := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started, *actor.Stopping, *actor.Stopped:
			// lifecycle
		default:
			ctx.Respond(ctx.Message())
		}
	})

	kind := NewKind("TestGrain", testKindProps)
	cp := newInmemoryProvider()
	c := newClusterForTest("trace-span-test", cp, WithKinds(kind))

	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	// Publish topology so the cluster knows about itself
	cp.publishClusterTopologyEvent()

	// Wait for cluster to initialize and gossip to run
	time.Sleep(3 * time.Second)

	// Make a cluster request that exercises the traced code paths
	msg := &struct{ Value string }{Value: "hello"}
	resp, err := c.Request("test-actor-1", "TestGrain", msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// Verify cluster.request span exists with correct attributes
	var foundClusterRequest, foundResolvePid bool
	var clusterRequestSpanID string
	var resolvePidParentSpanID string

	for _, s := range spans {
		switch s.Name {
		case "cluster.request":
			foundClusterRequest = true
			clusterRequestSpanID = s.SpanContext.SpanID().String()
			for _, attr := range s.Attributes {
				if string(attr.Key) == "kind" {
					assert.Equal(t, "TestGrain", attr.Value.AsString())
				}
				if string(attr.Key) == "identity" {
					assert.Equal(t, "test-actor-1", attr.Value.AsString())
				}
			}
		case "cluster.resolve_pid":
			foundResolvePid = true
			if s.Parent.IsValid() {
				resolvePidParentSpanID = s.Parent.SpanID().String()
			}
		}
	}

	assert.True(t, foundClusterRequest, "Expected cluster.request span")
	assert.True(t, foundResolvePid, "Expected cluster.resolve_pid span")

	if foundClusterRequest && foundResolvePid {
		assert.Equal(t, clusterRequestSpanID, resolvePidParentSpanID,
			"cluster.resolve_pid should be a child of cluster.request")
	}

	// Log gossip spans found (timing-dependent, so just informational)
	var gossipSpanNames []string
	for _, s := range spans {
		if s.Name == "gossip.send" || s.Name == "gossip.send_round" || s.Name == "gossip.receive" {
			gossipSpanNames = append(gossipSpanNames, s.Name)
		}
	}
	t.Logf("Gossip spans found: %v", gossipSpanNames)
}
