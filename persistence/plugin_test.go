package persistence

import (
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Use some common types from persistence example to setup
test cases
*/

const ActorName = "demo.actor"

var system = actor.NewActorSystem()

type dataStore struct {
	providerState ProviderState
}

// initData sets up a data store
// it adds one event to set state for every sting passed in
// set the last snapshot to given index of those events
func initData(snapshotInterval, lastSnapshot int, states ...string) *dataStore {
	// add all events
	state := NewInMemoryProvider(snapshotInterval)
	for i, s := range states {
		state.PersistEvent(ActorName, i, newMessage(s))
	}
	// mark one as a snapshot
	if lastSnapshot < len(states) {
		snapshot := states[lastSnapshot]
		state.PersistSnapshot(
			ActorName, lastSnapshot, newSnapshot(snapshot),
		)
	}
	return &dataStore{providerState: state}
}

func (p *dataStore) GetState() ProviderState {
	return p.providerState
}

type protoMsg struct {
	proto.Message
	state string
}

func (p *protoMsg) Reset()         {}
func (p *protoMsg) String() string { return p.state }
func (p *protoMsg) ProtoMessage()  {}

type (
	Message  struct{ protoMsg }
	Snapshot struct{ protoMsg }
	Query    struct{ protoMsg }
)

func newMessage(state string) *Message {
	return &Message{protoMsg: protoMsg{state: state}}
}

func newSnapshot(state string) *Snapshot {
	return &Snapshot{protoMsg: protoMsg{state: state}}
}

type myActor struct {
	Mixin
	state string
}

var _ actor.Actor = (*myActor)(nil)

func makeActor() actor.Actor {
	return &myActor{}
}

func (a *myActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *RequestSnapshot:
		a.PersistSnapshot(newSnapshot(a.state))
	case *Snapshot:
		a.state = msg.state
	case *Message:
		if !a.Recovering() {
			a.PersistReceive(msg)
		}
		a.state = msg.state
	case *Query:
		ctx.Respond(newMessage(a.state))
	}
}

/****** test code *******/

func TestRecovery(t *testing.T) {
	cases := []struct {
		init      *dataStore
		msgs      []string
		afterMsgs string
	}{
		// replay with no state
		0: {initData(5, 0), nil, ""},

		// replay directly on snapshot, no more messages
		1: {initData(8, 2, "a", "b", "c"), nil, "c"},

		// replay with snapshot and events, add another event
		2: {initData(8, 1, "a", "b", "c"), []string{"d"}, "d"},

		// replay state and add an event, which triggers snapshot
		3: {initData(4, 1, "a", "b", "c"), []string{"d"}, "d"},

		// replay state and add an event, which triggers snapshot,
		// and then another one
		4: {initData(4, 1, "a", "b", "c"), []string{"d", "e"}, "e"},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			rootContext := system.Root
			props := actor.PropsFromProducer(makeActor,
				actor.WithReceiverMiddleware(Using(tc.init)))
			pid, err := rootContext.SpawnNamed(props, ActorName)
			require.NoError(t, err)

			// send a bunch of messages
			for _, msg := range tc.msgs {
				rootContext.Send(pid, newMessage(msg))
			}

			resp, err := rootContext.RequestFuture(pid, &Query{}, 5*time.Second).Result()
			require.NoError(t, err)
			result, ok := resp.(*Message)
			require.True(t, ok)
			assert.Equal(t, tc.afterMsgs, result.state)

			// wait for shutdown
			_ = rootContext.PoisonFuture(pid).Wait()

			pid, err = rootContext.SpawnNamed(props, ActorName)
			require.NoError(t, err)

			resp, err = rootContext.RequestFuture(pid, &Query{}, 5*time.Second).Result()
			require.NoError(t, err)
			result, ok = resp.(*Message)
			require.True(t, ok)
			assert.Equal(t, tc.afterMsgs, result.state)

			// shutdown at end of test for cleanup
			_ = rootContext.PoisonFuture(pid).Wait()
		})
	}
}
