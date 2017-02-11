package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type CreateChildMessage struct{}
type GetChildCountRequest struct{}
type GetChildCountResponse struct{ ChildCount int }
type CreateChildActor struct{}

func (*CreateChildActor) Receive(context Context) {
	switch context.Message().(type) {
	case CreateChildMessage:
		context.Spawn(FromProducer(NewBlackHoleActor))
	case GetChildCountRequest:
		reply := GetChildCountResponse{ChildCount: len(context.Children())}
		context.Respond(reply)
	}
}

func NewCreateChildActor() Actor {
	return &CreateChildActor{}
}

func TestActorCanCreateChildren(t *testing.T) {
	a := Spawn(FromProducer(NewCreateChildActor))
	defer a.Stop()
	expected := 10
	for i := 0; i < expected; i++ {
		a.Tell(CreateChildMessage{})
	}
	future := a.RequestFuture(GetChildCountRequest{}, testTimeout)
	response := assertFutureSuccess(future,t)
	assert.Equal(t, expected, response.(GetChildCountResponse).ChildCount)
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
		context.Spawn(FromProducer(NewBlackHoleActor))
	case GetChildCountMessage2:
		msg.ReplyDirectly.Tell(true)
		state.replyTo = msg.ReplyAfterStop
	case *Stopped:
		reply := GetChildCountResponse{ChildCount: len(context.Children())}
		state.replyTo.Tell(reply)
	}
}

func NewCreateChildThenStopActor() Actor {
	return &CreateChildThenStopActor{}
}

func TestActorCanStopChildren(t *testing.T) {

	actor := Spawn(FromProducer(NewCreateChildThenStopActor))
	count := 10
	for i := 0; i < count; i++ {
		actor.Tell(CreateChildMessage{})
	}

	future := NewFuture(testTimeout)
	future2 := NewFuture(testTimeout)
	actor.Tell(GetChildCountMessage2{ReplyDirectly: future.PID(), ReplyAfterStop: future2.PID()})

	//wait for the actor to reply to the first responsePID
	assertFutureSuccess(future,t)

	//then send a stop command
	actor.Stop()

	//wait for the actor to stop and get the result from the stopped handler
	response := assertFutureSuccess(future2,t)
	//we should have 0 children when the actor is stopped
	assert.Equal(t, 0, response.(GetChildCountResponse).ChildCount)
}

func TestActorReceivesTerminatedFromWatched(t *testing.T) {
	child := Spawn(FromInstance(nullReceive))
	future := NewFuture(testTimeout)
	var wg sync.WaitGroup
	wg.Add(1)

	var r ActorFunc = func(c Context) {
		switch msg := c.Message().(type) {
		case *Started:
			c.Watch(child)
			wg.Done()

		case *Terminated:
			ac := c.(*localContext)
			if msg.Who.Equal(child) && ac.watching.Empty() {
				future.PID().Tell(true)
			}
		}
	}

	Spawn(FromInstance(r))
	wg.Wait()
	child.Stop()

	assertFutureSuccess(future,t)
}

func TestFutureDoesTimeout(t *testing.T) {
	pid := Spawn(FromInstance(nullReceive))
	_, err := pid.RequestFuture("", time.Millisecond).Result()
	assert.EqualError(t, err, ErrTimeout.Error())
}

func TestFutureDoesNotTimeout(t *testing.T) {
	var r ActorFunc = func(c Context) {
		if _, ok := c.Message().(string); !ok {
			return
		}

		time.Sleep(50 * time.Millisecond)
		c.Respond("foo")
	}
	pid := Spawn(FromInstance(r))
	reply, err := pid.RequestFuture("", 2*time.Second).Result()
	assert.NoError(t, err)
	assert.Equal(t, "foo", reply)
}
