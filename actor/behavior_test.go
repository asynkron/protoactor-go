package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type BehaviorMessage struct{}

type EchoSetBehaviorActor struct {
	behavior Behavior
}

func NewEchoBehaviorActor() Actor {
	state := &EchoSetBehaviorActor{
		behavior: NewBehavior(),
	}
	state.behavior.Become(state.one)
	return state
}

func (state *EchoSetBehaviorActor) Receive(context Context) {
	state.behavior.Receive(context)
}

func (state *EchoSetBehaviorActor) one(context Context) {
	switch context.Message().(type) {
	case BehaviorMessage:
		state.behavior.Become(state.other)
	}
}

func (EchoSetBehaviorActor) other(context Context) {
	switch context.Message().(type) {
	case EchoRequest:
		context.Respond(EchoResponse{})
	}
}

func TestActorCanSetBehavior(t *testing.T) {
	pid, _ := EmptyRootContext.Spawn(PropsFromProducer(NewEchoBehaviorActor))
	defer pid.Stop()
	EmptyRootContext.Send(pid, BehaviorMessage{})
	fut := EmptyRootContext.RequestFuture(pid, EchoRequest{}, testTimeout)
	assertFutureSuccess(fut, t)
}

type PopBehaviorMessage struct{}

type EchoPopBehaviorActor struct {
	behavior Behavior
}

func NewEchoUnbecomeActor() Actor {
	state := &EchoSetBehaviorActor{
		behavior: NewBehavior(),
	}
	state.behavior.Become(state.one)
	return state
}

func (state *EchoPopBehaviorActor) Receive(context Context) {
	state.behavior.Receive(context)
}

func (state *EchoPopBehaviorActor) one(context Context) {
	switch context.Message().(type) {
	case BehaviorMessage:
		state.behavior.BecomeStacked(state.other)
	case EchoRequest:
		context.Respond(EchoResponse{})
	}
}

func (state *EchoPopBehaviorActor) other(context Context) {
	switch context.Message().(type) {
	case PopBehaviorMessage:
		state.behavior.UnbecomeStacked()
	}
}

func TestActorCanPopBehavior(t *testing.T) {
	a, err := EmptyRootContext.Spawn(PropsFromProducer(NewEchoUnbecomeActor))
	assert.NoError(t, err)
	EmptyRootContext.Send(a, BehaviorMessage{})
	EmptyRootContext.Send(a, PopBehaviorMessage{})
	fut := EmptyRootContext.RequestFuture(a, EchoRequest{}, testTimeout)
	assertFutureSuccess(fut, t)
}
