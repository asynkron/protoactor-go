package remote

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

// envelopeCarrier adapts actor.MessageEnvelope to propagation.TextMapCarrier
// so that OTEL propagation can inject/extract trace context from message headers.
type envelopeCarrier struct {
	envelope *actor.MessageEnvelope
}

func (c envelopeCarrier) Get(key string) string {
	if c.envelope == nil || c.envelope.Header == nil {
		return ""
	}
	return c.envelope.Header.Get(key)
}

func (c envelopeCarrier) Set(key, val string) {
	if c.envelope != nil {
		c.envelope.SetHeader(key, val)
	}
}

func (c envelopeCarrier) Keys() []string {
	if c.envelope == nil || c.envelope.Header == nil {
		return nil
	}
	return c.envelope.Header.Keys()
}

// testSenderMiddleware injects trace context from the active span into outgoing message headers.
func testSenderMiddleware(activeSpans map[string]trace.Span) actor.SenderMiddleware {
	return func(next actor.SenderFunc) actor.SenderFunc {
		return func(c actor.SenderContext, target *actor.PID, envelope *actor.MessageEnvelope) {
			if span, ok := activeSpans[c.Self().String()]; ok {
				ctx := trace.ContextWithSpan(context.Background(), span)
				otel.GetTextMapPropagator().Inject(ctx, envelopeCarrier{envelope})
			}
			next(c, target, envelope)
		}
	}
}

// testReceiverMiddleware extracts trace context from incoming message headers
// and starts a new span linked to the propagated context.
func testReceiverMiddleware(activeSpans map[string]trace.Span) actor.ReceiverMiddleware {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(c actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			ctx := otel.GetTextMapPropagator().Extract(context.Background(), envelopeCarrier{envelope})
			_, span := otel.Tracer("protoactor/test").Start(ctx, fmt.Sprintf("%T", envelope.Message))
			activeSpans[c.Self().String()] = span
			defer func() {
				span.End()
				delete(activeSpans, c.Self().String())
			}()
			next(c, envelope)
		}
	}
}

func TestTraceContextPropagatesAcrossNodes_FireAndForget(t *testing.T) {
	// Set up in-memory span exporter.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	// Save and restore global tracer provider and propagator to avoid test pollution.
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	}()

	// Shared span maps for the test middleware (one per node).
	spansA := make(map[string]trace.Span)
	spansB := make(map[string]trace.Span)

	// Create two actor systems.
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

	// Spawn a receiver actor on Node B with tracing middleware.
	received := make(chan bool, 1)
	propsB := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *emptypb.Empty:
			received <- true
		}
	},
		actor.WithReceiverMiddleware(testReceiverMiddleware(spansB)),
		actor.WithSenderMiddleware(testSenderMiddleware(spansB)),
	)

	_, err = systemB.Root.SpawnNamed(propsB, "receiver")
	require.NoError(t, err)

	// Build the remote PID for the receiver on Node B.
	remotePidB := actor.NewPID(systemB.Address(), "receiver")

	// Spawn a sender actor on Node A with tracing middleware.
	// Sending must happen from within an actor context so SenderMiddleware fires
	// and injects the trace context into the message envelope headers.
	propsA := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			ctx.Send(remotePidB, &emptypb.Empty{})
		}
	},
		actor.WithReceiverMiddleware(testReceiverMiddleware(spansA)),
		actor.WithSenderMiddleware(testSenderMiddleware(spansA)),
	)

	_, err = systemA.Root.SpawnNamed(propsA, "sender")
	require.NoError(t, err)

	// Wait for the message to be received on Node B.
	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for message on Node B")
	}

	// Flush all spans.
	require.NoError(t, tp.ForceFlush(context.Background()))

	// Verify spans were recorded.
	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "Expected spans to be recorded")

	// Find the receiver's span for the Empty message. It should have a valid parent
	// trace ID inherited from the sender's span on Node A, proving cross-node propagation.
	var emptySpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "*emptypb.Empty" {
			emptySpan = &spans[i]
			break
		}
	}
	require.NotNil(t, emptySpan, "Expected a span for *emptypb.Empty on the receiver")

	// The Empty span must have a parent (the sender's span context propagated via headers).
	assert.True(t, emptySpan.Parent.SpanID().IsValid(),
		"Receiver span should have a parent span ID from the sender")

	// Verify that the sender's Started span on Node A shares the same trace ID
	// as the receiver's Empty span on Node B.
	senderAddr := systemA.Address()
	var senderStartedSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "*actor.Started" &&
			spans[i].SpanContext.TraceID() == emptySpan.SpanContext.TraceID() {
			senderStartedSpan = &spans[i]
			break
		}
	}
	require.NotNil(t, senderStartedSpan,
		"Expected a *actor.Started span sharing the trace ID with the receiver's span")

	assert.Equal(t, senderStartedSpan.SpanContext.TraceID(), emptySpan.SpanContext.TraceID(),
		"Sender (Node A at %s) and receiver (Node B) spans must share the same trace ID", senderAddr)

	// The receiver span should be a child of the sender span.
	assert.Equal(t, senderStartedSpan.SpanContext.SpanID(), emptySpan.Parent.SpanID(),
		"Receiver span should be a direct child of the sender span")
}
