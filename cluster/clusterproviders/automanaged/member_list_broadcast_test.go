package automanaged

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

func TestMemberList_Broadcast(t *testing.T) {
	c := startNode()
	defer c.Shutdown(true)

	var receivedEvent *cluster.GrainRequest

	for i := 1; i < 20; i++ { // retry several times as we don't know when the cluster will be ready
		var ok bool
		if receivedEvent, ok = trySendAndReceiveMessage(t, c, 0xBEEF); ok {
			break
		}
	}

	assert.Equal(t, int32(0xBEEF), receivedEvent.MethodIndex)
}

func startNode() *cluster.Cluster {
	system := actor.NewActorSystem()

	provider := New()
	config := remote.Configure("localhost", 0)

	lookup := disthash.New()
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	cluster := cluster.New(system, clusterConfig)

	cluster.StartMember()

	return cluster
}

func subscribe(c *cluster.Cluster) (events <-chan *cluster.GrainRequest, cancel func()) {
	eventChan := make(chan *cluster.GrainRequest, 1)

	subscription := c.ActorSystem.EventStream.Subscribe(func(evt interface{}) {
		if event, ok := evt.(*cluster.GrainRequest); ok {
			eventChan <- event
		}
	})

	return eventChan, func() { c.ActorSystem.EventStream.Unsubscribe(subscription) }
}

func trySendAndReceiveMessage(t *testing.T, c *cluster.Cluster, methodIndex int32) (receivedEvent *cluster.GrainRequest, ok bool) {
	events, cancel := subscribe(c)

	time.Sleep(500 * time.Millisecond)

	c.MemberList.BroadcastEvent(&cluster.GrainRequest{MethodIndex: methodIndex}, true)

	select {
	case receivedEvent = <-events:
		ok = true
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for the event to arrive")
	}

	cancel()

	return
}
