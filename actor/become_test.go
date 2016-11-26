package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type BecomeMessage struct{}

type EchoBecomeActor struct{}

func NewEchoBecomeActor() Actor {
	return &EchoBecomeActor{}
}

func (state *EchoBecomeActor) Receive(context Context) {
	switch context.Message().(type) {
	case BecomeMessage:
		context.Become(state.Other)
	}
}

func (EchoBecomeActor) Other(context Context) {
	switch context.Message().(type) {
	case EchoRequest:
		context.Respond(EchoResponse{})
	}
}

func TestActorCanBecome(t *testing.T) {
	actor := Spawn(FromProducer(NewEchoBecomeActor))
	defer actor.Stop()
	actor.Tell(BecomeMessage{})
	result := actor.RequestFuture(EchoRequest{}, testTimeout)
	if _, err := result.Result(); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type UnbecomeMessage struct{}

type EchoUnbecomeActor struct{}

func NewEchoUnbecomeActor() Actor {
	return &EchoBecomeActor{}
}

func (state *EchoUnbecomeActor) Receive(context Context) {
	switch context.Message().(type) {
	case BecomeMessage:
		context.BecomeStacked(state.Other)
	case EchoRequest:
		context.Respond(EchoResponse{})
	}
}

func (*EchoUnbecomeActor) Other(context Context) {
	switch context.Message().(type) {
	case UnbecomeMessage:
		context.UnbecomeStacked()
	}
}

func TestActorCanUnbecome(t *testing.T) {
	actor := Spawn(FromProducer(NewEchoUnbecomeActor))
	actor.Tell(BecomeMessage{})
	actor.Tell(UnbecomeMessage{})
	result := actor.RequestFuture(EchoRequest{}, testTimeout)
	if _, err := result.Result(); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}
