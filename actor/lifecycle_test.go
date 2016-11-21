package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type EchoOnStartActor struct{ replyTo *PID }

func (state *EchoOnStartActor) Receive(context Context) {
	switch context.Message().(type) {
	case *Started:
		state.replyTo.Tell(EchoReplyMessage{})
	}
}

func NewEchoOnStartActor(replyTo *PID) func() Actor {
	return func() Actor {
		return &EchoOnStartActor{replyTo: replyTo}
	}
}

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFuture(testTimeout)
	actor := Spawn(FromProducer(NewEchoOnStartActor(future.PID())))
	defer actor.Stop()
	if _, err := future.Result(); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type EchoOnStoppingActor struct{ replyTo *PID }

func (state *EchoOnStoppingActor) Receive(context Context) {
	switch context.Message().(type) {
	case *Stopping:
		state.replyTo.Tell(EchoReplyMessage{})
	}
}

func NewEchoOnStoppingActor(replyTo *PID) func() Actor {
	return func() Actor {
		return &EchoOnStoppingActor{replyTo: replyTo}
	}
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFuture(testTimeout)
	actor := Spawn(FromProducer(NewEchoOnStoppingActor(future.PID())))
	actor.Stop()
	if _, err := future.Result(); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}
