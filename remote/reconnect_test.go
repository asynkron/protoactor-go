package remote

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
)

// simulatePartition stops the given remote server to mimic a network partition.
func simulatePartition(r *Remote) {
	r.Shutdown(true)
}

// reconnectRemote starts the provided remote server after a partition.
func reconnectRemote(r *Remote) {
	r.Start()
}

// TestRemote_ReconnectAfterPartition starts a remote node, shuts it down to
// simulate a partition, and verifies communication after a new node comes back.
func TestRemote_ReconnectAfterPartition(t *testing.T) {
	systemA := actor.NewActorSystem()
	remoteA := NewRemote(systemA, Configure("127.0.0.1", 0))
	remoteA.Start()
	defer remoteA.Shutdown(true)

	// initial remote node
	systemB := actor.NewActorSystem()
	remoteB := NewRemote(systemB, Configure("127.0.0.1", 0))
	remoteB.Start()

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	pid1, err := systemB.Root.SpawnNamed(props, "echo1")
	assert.NoError(t, err)

	fut := systemA.Root.RequestFuture(pid1, &emptypb.Empty{}, time.Second)
	_, err = fut.Result()
	assert.NoError(t, err)

	// partition the first node
	simulatePartition(remoteB)

	fut = systemA.Root.RequestFuture(pid1, &emptypb.Empty{}, 200*time.Millisecond)
	_, err = fut.Result()
	assert.Error(t, err)

	// bring up a new remote node
	systemB2 := actor.NewActorSystem()
	remoteB2 := NewRemote(systemB2, Configure("127.0.0.1", 0))
	reconnectRemote(remoteB2)
	defer remoteB2.Shutdown(true)

	pid2, err := systemB2.Root.SpawnNamed(props, "echo2")
	assert.NoError(t, err)

	fut = systemA.Root.RequestFuture(pid2, &emptypb.Empty{}, time.Second)
	_, err = fut.Result()
	assert.NoError(t, err)
}
