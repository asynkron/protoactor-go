package actor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type DummyMessage struct{}
type BlackHoleActor struct{}

var testTimeout = 1 * time.Second

func (state *BlackHoleActor) Receive(context Context) {}

func NewBlackHoleActor() Actor {
	return &BlackHoleActor{}
}

func TestActorOfProducesActorRef(t *testing.T) {
	actor := ActorOf(Props(NewBlackHoleActor))
    defer actor.Stop()
	assert.NotNil(t, actor)
}

type FuncActor struct{ fun func() }

func NewFuncActor(fun func()) func() Actor {
	return func() Actor {
		return &FuncActor{fun: fun}
	}
}

func (state *FuncActor) Receive(context Context) {
	switch context.Message().(type) {
	case DummyMessage:
		state.fun()
	}
}

func TestActorReceivesMessage(t *testing.T) {
	done := make(chan struct{})
	actor := ActorOf(Props(NewFuncActor(func() { close(done) })))
    defer actor.Stop()
	actor.Tell(DummyMessage{})
	select {
	case <-done:
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}

type EchoRequest struct{ Sender ActorRef }

type EchoResponse struct{}

type EchoActor struct{}

func NewEchoActor() Actor {
	return &EchoActor{}
}

func (EchoActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case EchoRequest:
		msg.Sender.Tell(EchoResponse{})
	}
}

func TestActorCanReplyToMessage(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
    defer actor.Stop()
	actor.Tell(EchoRequest{Sender: future})
	select {
	case <-future.Result():
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}

type BecomeRequest struct{}

type EchoBecomeActor struct{}

func NewEchoBecomeActor() Actor {
	return &EchoBecomeActor{}
}

func (state EchoBecomeActor) Receive(context Context) {
	switch context.Message().(type) {
	case BecomeRequest:
		context.Become(state.Other)
	}
}

func (EchoBecomeActor) Other(context Context) {
	switch msg := context.Message().(type) {
	case EchoRequest:
		msg.Sender.Tell(EchoResponse{})
	}
}

func TestActorCanBecome(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
    defer actor.Stop()
	actor.Tell(BecomeRequest{})
	actor.Tell(EchoRequest{Sender: future})
	select {
	case <-future.Result():
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}

type UnbecomeRequest struct{}

type EchoUnbecomeActor struct{}

func NewEchoUnbecomeActor() Actor {
	return &EchoBecomeActor{}
}

func (state EchoUnbecomeActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case BecomeRequest:
		context.BecomeStacked(state.Other)
	case EchoRequest:
		msg.Sender.Tell(EchoResponse{})
	}
}

func (EchoUnbecomeActor) Other(context Context) {
	switch context.Message().(type) {
	case UnbecomeRequest:
		context.UnbecomeStacked()
	}
}

func TestActorCanUnbecome(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
    defer actor.Stop()
	actor.Tell(BecomeRequest{})
	actor.Tell(UnbecomeRequest{})
	actor.Tell(EchoRequest{Sender: future})
	select {
	case <-future.Result():
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}

type EchoOnStartActor struct{replyTo ActorRef}

func (state EchoOnStartActor) Receive(context Context) {
	switch context.Message().(type) {
	case Starting:
		state.replyTo.Tell(EchoResponse{})
	}
}

func NewEchoOnStartActor(replyTo ActorRef) func() Actor {
    return func() Actor{
	    return &EchoOnStartActor{replyTo: replyTo}
    }
}

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoOnStartActor(future)))
    defer actor.Stop()
	select {
	case <-future.Result():
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}


type EchoOnStoppingActor struct{replyTo ActorRef}

func (state EchoOnStoppingActor) Receive(context Context) {
	switch context.Message().(type) {
	case Stopping:
		state.replyTo.Tell(EchoResponse{})
	}
}

func NewEchoOnStoppingActor(replyTo ActorRef) func() Actor {
    return func() Actor{
	    return &EchoOnStoppingActor{replyTo: replyTo}
    }
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoOnStoppingActor(future)))
    actor.Stop()
	select {
	case <-future.Result():
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}