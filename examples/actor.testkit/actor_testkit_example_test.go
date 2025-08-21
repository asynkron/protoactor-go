package actortestkit

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/testkit"
	"github.com/stretchr/testify/require"
)

// TestUsingTestProbe shows how to interact with actors via TestProbe.
func TestUsingTestProbe(t *testing.T) {
	system := actor.NewActorSystem()

	// Spawn the probe as an actor in the system and wait for it to be ready.
	probe := testkit.NewTestProbe()
	probePID := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor { return probe }))
	system.Root.Send(probePID, "ready")
	_, err := testkit.GetNextMessageOf[string](probe, time.Second)
	require.NoError(t, err)

	// Echo actor replies with "pong" to any string message.
	echoProps := actor.PropsFromFunc(func(ctx actor.Context) {
		if msg, ok := ctx.Message().(string); ok {
			ctx.Respond("pong: " + msg)
		}
	})
	echoPID := system.Root.Spawn(echoProps)

	// Use the probe to send a request and assert on the response.
	probe.Request(echoPID, "ping")
	reply, err := testkit.GetNextMessageOf[string](probe, time.Second)
	require.NoError(t, err)
	require.Equal(t, "pong: ping", reply)

	// Ensure no unexpected messages arrive.
	require.NoError(t, probe.ExpectNoMessage(50*time.Millisecond))
}

// TestUsingTestMailboxStats demonstrates capturing mailbox activity.
func TestUsingTestMailboxStats(t *testing.T) {
	system := actor.NewActorSystem()

	// Collect mailbox events and signal when "done" is received.
	stats := testkit.NewTestMailboxStats(func(msg interface{}) bool { return msg == "done" })
	// Attach the collector directly to the mailbox via props.
	props := actor.PropsFromFunc(func(ctx actor.Context) {}, testkit.WithMailboxStats(stats))
	pid := system.Root.Spawn(props)

	// Send a message and wait until it is processed.
	system.Root.Send(pid, "done")
	select {
	case <-stats.Reset:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	// Allow time for mailbox events to be recorded.
	time.Sleep(50 * time.Millisecond)

	// The first posted/received message is *actor.Started; the second is our message.
	require.Len(t, stats.Posted, 2)
	require.Equal(t, "done", stats.Posted[1])
	require.Len(t, stats.Received, 2)
	require.Equal(t, "done", stats.Received[1])
}

// TestMailboxStatsAsReceiverMiddleware shows using stats as a receive middleware.
func TestMailboxStatsAsReceiverMiddleware(t *testing.T) {
	system := actor.NewActorSystem()

	// Signal when "done" is observed by the middleware.
	stats := testkit.NewTestMailboxStats(func(msg interface{}) bool { return msg == "done" })
	props := actor.PropsFromFunc(func(ctx actor.Context) {}, testkit.WithReceiveStats(stats))
	pid := system.Root.Spawn(props)

	// Trigger the actor and wait for the middleware to see the message.
	system.Root.Send(pid, "done")
	select {
	case <-stats.Reset:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	time.Sleep(50 * time.Millisecond)

	// *actor.Started is first, followed by our message.
	require.Len(t, stats.Received, 2)
	require.Equal(t, "done", stats.Received[1])
}

// TestMailboxStatsAsSendMiddleware shows using stats as a send middleware.
func TestMailboxStatsAsSendMiddleware(t *testing.T) {
	system := actor.NewActorSystem()

	// No wait predicate needed as we inspect Posted after the fact.
	stats := testkit.NewTestMailboxStats(nil)
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if msg, ok := ctx.Message().(string); ok && msg == "start" {
			ctx.Respond("done")
		}
	}, testkit.WithSendStats(stats))
	pid := system.Root.Spawn(props)

	// Request a response to trigger sending.
	res, err := system.Root.RequestFuture(pid, "start", time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, "done", res)

	time.Sleep(50 * time.Millisecond)

	require.Len(t, stats.Posted, 1)
	require.Equal(t, "done", stats.Posted[0])
}
