package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type BehaviorMessage struct{}

type EchoSetBehaviorActor struct{}

func NewEchoBehaviorActor() Actor {
	return &EchoSetBehaviorActor{}
}

func (state *EchoSetBehaviorActor) Receive(context Context) {
	switch context.Message().(type) {
	case BehaviorMessage:
		context.SetBehavior(state.Other)
	}
}

func (EchoSetBehaviorActor) Other(context Context) {
	switch context.Message().(type) {
	case EchoRequest:
		context.Respond(EchoResponse{})
	}
}

func TestActorCanSetBehavior(t *testing.T) {
	actor := Spawn(FromProducer(NewEchoBehaviorActor))
	defer actor.Stop()
	actor.Tell(BehaviorMessage{})
	result := actor.RequestFuture(EchoRequest{}, testTimeout)
	if _, err := result.Result(); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type PopBehaviorMessage struct{}

type EchoPopBehaviorActor struct{}

func NewEchoUnbecomeActor() Actor {
	return &EchoSetBehaviorActor{}
}

func (state *EchoPopBehaviorActor) Receive(context Context) {
	switch context.Message().(type) {
	case BehaviorMessage:
		context.PushBehavior(state.Other)
	case EchoRequest:
		context.Respond(EchoResponse{})
	}
}

func (*EchoPopBehaviorActor) Other(context Context) {
	switch context.Message().(type) {
	case PopBehaviorMessage:
		context.PopBehavior()
	}
}

func TestActorCanPopBehavior(t *testing.T) {
	actor := Spawn(FromProducer(NewEchoUnbecomeActor))
	actor.Tell(BehaviorMessage{})
	actor.Tell(PopBehaviorMessage{})
	result := actor.RequestFuture(EchoRequest{}, testTimeout)
	if _, err := result.Result(); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}
