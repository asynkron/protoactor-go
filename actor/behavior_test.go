package actor

import (
	"testing"
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
	pid := Spawn(FromProducer(NewEchoBehaviorActor))
	defer pid.Stop()
	pid.Tell(BehaviorMessage{})
	fut := pid.RequestFuture(EchoRequest{}, testTimeout)
	assertFutureSuccess(fut, t)
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
	a := Spawn(FromProducer(NewEchoUnbecomeActor))
	a.Tell(BehaviorMessage{})
	a.Tell(PopBehaviorMessage{})
	fut := a.RequestFuture(EchoRequest{}, testTimeout)
	assertFutureSuccess(fut, t)
}
