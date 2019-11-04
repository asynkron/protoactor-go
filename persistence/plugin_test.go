package persistence

import (
	"fmt"
	"sync"
	"testing"

	"github.com/otherview/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Use some common types from persistence example to setup
test cases
*/

const ActorName = "demo.actor"

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

type protoMsg struct{ state string }

func (p *protoMsg) Reset()         {}
func (p *protoMsg) String() string { return p.state }
func (p *protoMsg) ProtoMessage()  {}

type Message struct{ protoMsg }
type Snapshot struct{ protoMsg }
type Query struct{ protoMsg }

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

var queryWg sync.WaitGroup
var queryState string

func (a *myActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *RequestSnapshot:
		// PersistSnapshot when requested
		a.PersistSnapshot(newSnapshot(a.state))
	case *Snapshot:
		// Restore from Snapshot
		a.state = msg.state
	case *Message:
		// Persist all events received outside of recovery
		if !a.Recovering() {
			a.PersistReceive(msg)
		}
		// Set state to whatever message says
		a.state = msg.state
	case *Query:
		// TODO: this is poorly writen...
		// I have no idea how to synchronously block on the
		// receipt of a message for test cases.
		queryState = a.state
		queryWg.Done()
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
			rootContext := actor.EmptyRootContext
			props := actor.PropsFromProducer(makeActor).
				WithReceiverMiddleware(Using(tc.init))
			pid, err := rootContext.SpawnNamed(props, ActorName)
			require.NoError(t, err)

			// send a bunch of messages
			for _, msg := range tc.msgs {
				rootContext.Send(pid, newMessage(msg))
			}

			// ugly way to block on a response....
			// TODO: I need some help here
			queryWg.Add(1)
			rootContext.Send(pid, &Query{})
			queryWg.Wait()
			// check the state after all these messages
			assert.Equal(t, tc.afterMsgs, queryState)

			// wait for shutdown
			rootContext.PoisonFuture(pid).Wait()

			pid, err = rootContext.SpawnNamed(props, ActorName)
			require.NoError(t, err)

			// ugly way to block on a response....
			// TODO: I need some help here
			queryWg.Add(1)
			rootContext.Send(pid, &Query{})
			queryWg.Wait()
			// check the state after all these messages
			assert.Equal(t, tc.afterMsgs, queryState)

			// shutdown at end of test for cleanup
			rootContext.PoisonFuture(pid).Wait()
		})
	}
}
