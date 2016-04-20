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

type EchoMessage struct{ Sender ActorRef }

type EchoReplyMessage struct{}

type EchoActor struct{}

func NewEchoActor() Actor {
	return &EchoActor{}
}

func (*EchoActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case EchoMessage:
		msg.Sender.Tell(EchoReplyMessage{})
	}
}

func TestActorCanReplyToMessage(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
	defer actor.Stop()
	actor.Tell(EchoMessage{Sender: future})
	if _, err := future.ResultOrTimeout(testTimeout); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

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
	switch msg := context.Message().(type) {
	case EchoMessage:
		msg.Sender.Tell(EchoReplyMessage{})
	}
}

func TestActorCanBecome(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
	defer actor.Stop()
	actor.Tell(BecomeMessage{})
	actor.Tell(EchoMessage{Sender: future})
	if _, err := future.ResultOrTimeout(testTimeout); err != nil {
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
	switch msg := context.Message().(type) {
	case BecomeMessage:
		context.BecomeStacked(state.Other)
	case EchoMessage:
		msg.Sender.Tell(EchoReplyMessage{})
	}
}

func (*EchoUnbecomeActor) Other(context Context) {
	switch context.Message().(type) {
	case UnbecomeMessage:
		context.UnbecomeStacked()
	}
}

func TestActorCanUnbecome(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoActor))
	defer actor.Stop()
	actor.Tell(BecomeMessage{})
	actor.Tell(UnbecomeMessage{})
	actor.Tell(EchoMessage{Sender: future})
	if _, err := future.ResultOrTimeout(testTimeout); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type EchoOnStartActor struct{ replyTo ActorRef }

func (state *EchoOnStartActor) Receive(context Context) {
	switch context.Message().(type) {
	case Started:
		state.replyTo.Tell(EchoReplyMessage{})
	}
}

func NewEchoOnStartActor(replyTo ActorRef) func() Actor {
	return func() Actor {
		return &EchoOnStartActor{replyTo: replyTo}
	}
}

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoOnStartActor(future)))
	defer actor.Stop()
	if _, err := future.ResultOrTimeout(testTimeout); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type EchoOnStoppingActor struct{ replyTo ActorRef }

func (state *EchoOnStoppingActor) Receive(context Context) {
	switch context.Message().(type) {
	case Stopping:
		state.replyTo.Tell(EchoReplyMessage{})
	}
}

func NewEchoOnStoppingActor(replyTo ActorRef) func() Actor {
	return func() Actor {
		return &EchoOnStoppingActor{replyTo: replyTo}
	}
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewEchoOnStoppingActor(future)))
	actor.Stop()
	if _, err := future.ResultOrTimeout(testTimeout); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type CreateChildMessage struct{}
type GetChildCountMessage struct{ ReplyTo ActorRef }
type GetChildCountReplyMessage struct{ ChildCount int }
type CreateChildActor struct{}

func (*CreateChildActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case CreateChildMessage:
		context.ActorOf(Props(NewBlackHoleActor))
	case GetChildCountMessage:
		reply := GetChildCountReplyMessage{ChildCount: len(context.Children())}
		msg.ReplyTo.Tell(reply)
	}
}

func NewCreateChildActor() Actor {
	return &CreateChildActor{}
}

func TestActorCanCreateChildren(t *testing.T) {
	future := NewFutureActorRef()
	actor := ActorOf(Props(NewCreateChildActor))
	defer actor.Stop()
	expected := 10
	for i := 0; i < expected; i++ {
		actor.Tell(CreateChildMessage{})
	}
	actor.Tell(GetChildCountMessage{ReplyTo: future})
	response, err := future.ResultOrTimeout(testTimeout)
	if err != nil {
		assert.Fail(t, "timed out")
		return
	}
	assert.Equal(t, expected, response.(GetChildCountReplyMessage).ChildCount)
}

type CreateChildThenStopActor struct {
	replyTo ActorRef
}

type GetChildCountMessage2 struct {
	ReplyDirectly  ActorRef
	ReplyAfterStop ActorRef
}

func (state *CreateChildThenStopActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case CreateChildMessage:
		context.ActorOf(Props(NewBlackHoleActor))
	case GetChildCountMessage2:
		msg.ReplyDirectly.Tell(true)
		state.replyTo = msg.ReplyAfterStop
	case Stopped:
		reply := GetChildCountReplyMessage{ChildCount: len(context.Children())}
		state.replyTo.Tell(reply)
	}
}

func NewCreateChildThenStopActor() Actor {
	return &CreateChildThenStopActor{}
}

func TestActorCanStopChildren(t *testing.T) {
	future := NewFutureActorRef()
	afterStopped := NewFutureActorRef()
	actor := ActorOf(Props(NewCreateChildThenStopActor))
	count := 10
	for i := 0; i < count; i++ {
		actor.Tell(CreateChildMessage{})
	}
	actor.Tell(GetChildCountMessage2{ReplyDirectly: future, ReplyAfterStop: afterStopped})

	//wait for the actor to reply to the first future
	_, err := future.ResultOrTimeout(testTimeout)
	if err != nil {
		assert.Fail(t, "timed out")
		return
	}

	//then send a stop command
	actor.Stop()

	//wait for the actor to stop and get the result from the stopped handler
	response, err := afterStopped.ResultOrTimeout(testTimeout)
	if err != nil {
		assert.Fail(t, "timed out")
		return
	}
	//we should have 0 children when the actor is stopped
	assert.Equal(t, 0, response.(GetChildCountReplyMessage).ChildCount)
}
