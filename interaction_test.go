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
	actor.Tell(DummyMessage{})
	select {
	case <-done:
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}

type Echo struct{ Sender ActorRef }

type EchoEcho struct{}

type EchoActor struct{}

func NewEchoActor() Actor {
	return &EchoActor{}
}

func (EchoActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case Echo:
		msg.Sender.Tell(EchoEcho{})
	}
}

func TestActorCanReplyToMessage(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
	actor.Tell(Echo{Sender: future})
	select {
	case <-future.Result():
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}

type BecomeMessage struct{}

type EchoBecomeActor struct{}

func NewEchoBecomeActor() Actor {
	return &EchoBecomeActor{}
}

func (state EchoBecomeActor) Receive(context Context) {
	switch context.Message().(type) {
	case BecomeMessage:
		context.Become(state.Other)
	}
}

func (EchoBecomeActor) Other(context Context) {
	switch msg := context.Message().(type) {
	case Echo:
		msg.Sender.Tell(EchoEcho{})
	}
}

func TestActorCanBecome(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
    actor.Tell(BecomeMessage{})
	actor.Tell(Echo{Sender: future})
	select {
	case <-future.Result():
	case <-time.After(testTimeout):
		assert.Fail(t, "timed out")
	}
}