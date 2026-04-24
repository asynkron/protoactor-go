package remote

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
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

func TestTraceContextPropagatesAcrossNodes_RequestResponse(t *testing.T) {
	// Set up in-memory span exporter.
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

	spansA := make(map[string]trace.Span)
	spansB := make(map[string]trace.Span)

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

	// Spawn a responder actor on Node B that responds to requests.
	propsB := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *emptypb.Empty:
			ctx.Respond(&emptypb.Empty{})
		}
	},
		actor.WithReceiverMiddleware(testReceiverMiddleware(spansB)),
		actor.WithSenderMiddleware(testSenderMiddleware(spansB)),
	)

	_, err = systemB.Root.SpawnNamed(propsB, "responder")
	require.NoError(t, err)

	remotePidB := actor.NewPID(systemB.Address(), "responder")

	// Spawn a requester actor on Node A that sends a request and waits for the response.
	responseReceived := make(chan bool, 1)
	propsA := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			// Use Request so response comes back to this actor (through middleware).
			ctx.Request(remotePidB, &emptypb.Empty{})
		case *emptypb.Empty:
			// This is the response from Node B.
			responseReceived <- true
		}
	},
		actor.WithReceiverMiddleware(testReceiverMiddleware(spansA)),
		actor.WithSenderMiddleware(testSenderMiddleware(spansA)),
	)

	_, err = systemA.Root.SpawnNamed(propsA, "requester")
	require.NoError(t, err)

	select {
	case <-responseReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for response on Node A")
	}

	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "Expected spans to be recorded")

	// Collect all *emptypb.Empty spans (there should be at least 2: one on B for request, one on A for response).
	var emptySpans []tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "*emptypb.Empty" {
			emptySpans = append(emptySpans, spans[i])
		}
	}
	require.GreaterOrEqual(t, len(emptySpans), 2,
		"Expected at least 2 *emptypb.Empty spans (request on B, response on A)")

	// Find the sender's *actor.Started span that initiated the trace.
	var startedSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "*actor.Started" {
			// Pick the one that has *emptypb.Empty children (same trace ID).
			for _, es := range emptySpans {
				if spans[i].SpanContext.TraceID() == es.SpanContext.TraceID() {
					startedSpan = &spans[i]
					break
				}
			}
			if startedSpan != nil {
				break
			}
		}
	}
	require.NotNil(t, startedSpan, "Expected a *actor.Started span sharing the trace with Empty spans")

	traceID := startedSpan.SpanContext.TraceID()

	// All *emptypb.Empty spans in this trace should share the same trace ID.
	var traceEmptySpans []tracetest.SpanStub
	for _, es := range emptySpans {
		if es.SpanContext.TraceID() == traceID {
			traceEmptySpans = append(traceEmptySpans, es)
		}
	}
	require.GreaterOrEqual(t, len(traceEmptySpans), 2,
		"Expected at least 2 *emptypb.Empty spans in the same trace")

	// Find the request span on B (child of Started) and the response span on A (child of B's span).
	// The request span's parent should be the Started span.
	var requestSpan, responseSpan *tracetest.SpanStub
	for i := range traceEmptySpans {
		if traceEmptySpans[i].Parent.SpanID() == startedSpan.SpanContext.SpanID() {
			requestSpan = &traceEmptySpans[i]
		}
	}
	require.NotNil(t, requestSpan,
		"Expected a *emptypb.Empty span (request on B) whose parent is the Started span")

	for i := range traceEmptySpans {
		if traceEmptySpans[i].Parent.SpanID() == requestSpan.SpanContext.SpanID() {
			responseSpan = &traceEmptySpans[i]
		}
	}
	require.NotNil(t, responseSpan,
		"Expected a *emptypb.Empty span (response on A) whose parent is B's request span")

	// Verify complete chain: Started -> request (B) -> response (A), all same trace ID.
	assert.Equal(t, traceID, requestSpan.SpanContext.TraceID(),
		"Request span on B must share the trace ID")
	assert.Equal(t, traceID, responseSpan.SpanContext.TraceID(),
		"Response span on A must share the trace ID")
	assert.Equal(t, startedSpan.SpanContext.SpanID(), requestSpan.Parent.SpanID(),
		"Request span should be a child of Started span")
	assert.Equal(t, requestSpan.SpanContext.SpanID(), responseSpan.Parent.SpanID(),
		"Response span should be a child of request span")
}

func TestTraceContextPropagatesAcrossNodes_MultiHop(t *testing.T) {
	// Set up in-memory span exporter.
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

	spansA := make(map[string]trace.Span)
	spansB := make(map[string]trace.Span)
	spansC := make(map[string]trace.Span)

	systemA := actor.NewActorSystem()
	systemB := actor.NewActorSystem()
	systemC := actor.NewActorSystem()

	configA := Configure("127.0.0.1", 0)
	configB := Configure("127.0.0.1", 0)
	configC := Configure("127.0.0.1", 0)

	remoteA := NewRemote(systemA, configA)
	remoteB := NewRemote(systemB, configB)
	remoteC := NewRemote(systemC, configC)

	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(true)

	err = remoteC.Start()
	require.NoError(t, err)
	defer remoteC.Shutdown(true)

	// Spawn the final receiver on Node C.
	receivedOnC := make(chan bool, 1)
	propsC := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *emptypb.Empty:
			receivedOnC <- true
		}
	},
		actor.WithReceiverMiddleware(testReceiverMiddleware(spansC)),
		actor.WithSenderMiddleware(testSenderMiddleware(spansC)),
	)

	_, err = systemC.Root.SpawnNamed(propsC, "final-receiver")
	require.NoError(t, err)

	remotePidC := actor.NewPID(systemC.Address(), "final-receiver")

	// Spawn a relay actor on Node B that forwards messages to Node C.
	propsB := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *emptypb.Empty:
			ctx.Send(remotePidC, &emptypb.Empty{})
		}
	},
		actor.WithReceiverMiddleware(testReceiverMiddleware(spansB)),
		actor.WithSenderMiddleware(testSenderMiddleware(spansB)),
	)

	_, err = systemB.Root.SpawnNamed(propsB, "relay")
	require.NoError(t, err)

	remotePidB := actor.NewPID(systemB.Address(), "relay")

	// Spawn a sender actor on Node A.
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

	select {
	case <-receivedOnC:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for message on Node C")
	}

	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "Expected spans to be recorded")

	// Find the sender's *actor.Started span on Node A.
	// It should be the root of the trace that includes Empty spans on B and C.
	var emptySpans []tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "*emptypb.Empty" {
			emptySpans = append(emptySpans, spans[i])
		}
	}
	require.GreaterOrEqual(t, len(emptySpans), 2,
		"Expected at least 2 *emptypb.Empty spans (B relay and C final receiver)")

	var startedSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "*actor.Started" {
			for _, es := range emptySpans {
				if spans[i].SpanContext.TraceID() == es.SpanContext.TraceID() {
					startedSpan = &spans[i]
					break
				}
			}
			if startedSpan != nil {
				break
			}
		}
	}
	require.NotNil(t, startedSpan, "Expected a *actor.Started span sharing the trace with Empty spans")

	traceID := startedSpan.SpanContext.TraceID()

	// Collect all Empty spans in this trace.
	var traceEmptySpans []tracetest.SpanStub
	for _, es := range emptySpans {
		if es.SpanContext.TraceID() == traceID {
			traceEmptySpans = append(traceEmptySpans, es)
		}
	}
	require.GreaterOrEqual(t, len(traceEmptySpans), 2,
		"Expected at least 2 *emptypb.Empty spans in the same trace for the multi-hop")

	// Find the relay span on B (child of Started) and final span on C (child of B's span).
	var relaySpan *tracetest.SpanStub
	for i := range traceEmptySpans {
		if traceEmptySpans[i].Parent.SpanID() == startedSpan.SpanContext.SpanID() {
			relaySpan = &traceEmptySpans[i]
			break
		}
	}
	require.NotNil(t, relaySpan,
		"Expected a *emptypb.Empty span (relay on B) whose parent is the Started span on A")

	var finalSpan *tracetest.SpanStub
	for i := range traceEmptySpans {
		if traceEmptySpans[i].Parent.SpanID() == relaySpan.SpanContext.SpanID() {
			finalSpan = &traceEmptySpans[i]
			break
		}
	}
	require.NotNil(t, finalSpan,
		"Expected a *emptypb.Empty span (final on C) whose parent is the relay span on B")

	// Verify the complete parent chain: A (Started) -> B (relay Empty) -> C (final Empty).
	assert.Equal(t, traceID, relaySpan.SpanContext.TraceID(),
		"Relay span on B must share the trace ID")
	assert.Equal(t, traceID, finalSpan.SpanContext.TraceID(),
		"Final span on C must share the trace ID")
	assert.Equal(t, startedSpan.SpanContext.SpanID(), relaySpan.Parent.SpanID(),
		"Relay span should be a child of the Started span on A")
	assert.Equal(t, relaySpan.SpanContext.SpanID(), finalSpan.Parent.SpanID(),
		"Final span should be a child of the relay span on B")
}
