package testkit

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/require"
	"sync"
)

func TestMailboxStatsCapturesMessages(t *testing.T) {
	system := actor.NewActorSystem()
	stats := NewTestMailboxStats(func(msg interface{}) bool { return msg == "hi" })
	props := actor.PropsFromFunc(func(actor.Context) {}, actor.WithMailbox(actor.Unbounded(stats)))
	pid := system.Root.Spawn(props)

	system.Root.Send(pid, "hi")
	select {
	case <-stats.Reset:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	time.Sleep(50 * time.Millisecond)

	require.Len(t, stats.Posted, 2)
	require.Equal(t, "hi", stats.Posted[1])
	require.Len(t, stats.Received, 2)
	require.Equal(t, "hi", stats.Received[1])
}

func TestMailboxStatsConcurrent(t *testing.T) {
	stats := NewTestMailboxStats(nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(v int) {
			defer wg.Done()
			stats.MessagePosted(v)
		}(i)
		go func(v int) {
			defer wg.Done()
			stats.MessageReceived(v)
		}(i)
	}
	wg.Wait()
	require.Len(t, stats.Posted, 100)
	require.Len(t, stats.Received, 100)
}
