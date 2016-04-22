package gam

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

func TestSpawnProducesActorRef(t *testing.T) {
	actor := Spawn(Props(NewBlackHoleActor))
	defer actor.Stop()
	assert.NotNil(t, actor)
}

type EchoMessage struct{ Sender *PID }

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
	replyPID, result := FuturePID()
	actor := Spawn(Props(NewEchoActor))
	defer actor.Stop()
	actor.Tell(EchoMessage{Sender: replyPID})
	if _, err := result.ResultOrTimeout(testTimeout); err != nil {
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
	replyPID, result := FuturePID()
	actor := Spawn(Props(NewEchoActor))
	defer actor.Stop()
	actor.Tell(BecomeMessage{})
	actor.Tell(EchoMessage{Sender: replyPID})
	if _, err := result.ResultOrTimeout(testTimeout); err != nil {
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
	replyPID, result := FuturePID()
	actor := Spawn(Props(NewEchoActor))
	defer actor.Stop()
	actor.Tell(BecomeMessage{})
	actor.Tell(UnbecomeMessage{})
	actor.Tell(EchoMessage{Sender: replyPID})
	if _, err := result.ResultOrTimeout(testTimeout); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type EchoOnStartActor struct{ replyTo *PID }

func (state *EchoOnStartActor) Receive(context Context) {
	switch context.Message().(type) {
	case Started:
		state.replyTo.Tell(EchoReplyMessage{})
	}
}

func NewEchoOnStartActor(replyTo *PID) func() Actor {
	return func() Actor {
		return &EchoOnStartActor{replyTo: replyTo}
	}
}

// func TestActorCanReplyOnStarting(t *testing.T) {
// 	replyPID,result := FuturePID()
// 	actor := Spawn(Props(NewEchoOnStartActor(replyPID)))
// 	defer actor.Stop()
// 	if _, err := result.ResultOrTimeout(testTimeout); err != nil {
// 		assert.Fail(t, "timed out")
// 		return
// 	}
// }

type EchoOnStoppingActor struct{ replyTo *PID }

func (state *EchoOnStoppingActor) Receive(context Context) {
	switch context.Message().(type) {
	case Stopping:
		state.replyTo.Tell(EchoReplyMessage{})
	}
}

func NewEchoOnStoppingActor(replyTo *PID) func() Actor {
	return func() Actor {
		return &EchoOnStoppingActor{replyTo: replyTo}
	}
}

func TestActorCanReplyOnStopping(t *testing.T) {
	replyPID, result := FuturePID()
	actor := Spawn(Props(NewEchoOnStoppingActor(replyPID)))
	actor.Stop()
	if _, err := result.ResultOrTimeout(testTimeout); err != nil {
		assert.Fail(t, "timed out")
		return
	}
}

type CreateChildMessage struct{}
type GetChildCountMessage struct{ ReplyTo *PID }
type GetChildCountReplyMessage struct{ ChildCount int }
type CreateChildActor struct{}

func (*CreateChildActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case CreateChildMessage:
		context.Spawn(Props(NewBlackHoleActor))
	case GetChildCountMessage:
		reply := GetChildCountReplyMessage{ChildCount: len(context.Children())}
		msg.ReplyTo.Tell(reply)
	}
}

func NewCreateChildActor() Actor {
	return &CreateChildActor{}
}

func TestActorCanCreateChildren(t *testing.T) {
	replyPID, result := FuturePID()
	actor := Spawn(Props(NewCreateChildActor))
	defer actor.Stop()
	expected := 10
	for i := 0; i < expected; i++ {
		actor.Tell(CreateChildMessage{})
	}
	actor.Tell(GetChildCountMessage{ReplyTo: replyPID})
	response, err := result.ResultOrTimeout(testTimeout)
	if err != nil {
		assert.Fail(t, "timed out")
		return
	}
	assert.Equal(t, expected, response.(GetChildCountReplyMessage).ChildCount)
}

type CreateChildThenStopActor struct {
	replyTo *PID
}

type GetChildCountMessage2 struct {
	ReplyDirectly  *PID
	ReplyAfterStop *PID
}

func (state *CreateChildThenStopActor) Receive(context Context) {
	switch msg := context.Message().(type) {
	case CreateChildMessage:
		context.Spawn(Props(NewBlackHoleActor))
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
	replyPID, result := FuturePID()
	replyPID2, result2 := FuturePID()
	actor := Spawn(Props(NewCreateChildThenStopActor))
	count := 10
	for i := 0; i < count; i++ {
		actor.Tell(CreateChildMessage{})
	}
	actor.Tell(GetChildCountMessage2{ReplyDirectly: replyPID, ReplyAfterStop: replyPID2})

	//wait for the actor to reply to the first replyPID
	_, err := result.ResultOrTimeout(testTimeout)
	if err != nil {
		assert.Fail(t, "timed out")
		return
	}

	//then send a stop command
	actor.Stop()

	//wait for the actor to stop and get the result from the stopped handler
	response, err := result2.ResultOrTimeout(testTimeout)
	if err != nil {
		assert.Fail(t, "timed out")
		return
	}
	//we should have 0 children when the actor is stopped
	assert.Equal(t, 0, response.(GetChildCountReplyMessage).ChildCount)
}
