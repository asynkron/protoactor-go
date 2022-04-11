package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type (
	CreateChildMessage    struct{}
	GetChildCountRequest  struct{}
	GetChildCountResponse struct{ ChildCount int }
	CreateChildActor      struct{}
)

func (*CreateChildActor) Receive(context Context) {
	switch context.Message().(type) {
	case CreateChildMessage:
		context.Spawn(PropsFromProducer(NewBlackHoleActor))
	case GetChildCountRequest:
		reply := GetChildCountResponse{ChildCount: len(context.Children())}
		context.Respond(reply)
	}
}

func NewCreateChildActor() Actor {
	return &CreateChildActor{}
}

func TestActorCanCreateChildren(t *testing.T) {
	a := rootContext.Spawn(PropsFromProducer(NewCreateChildActor))
	defer rootContext.Stop(a)
	expected := 10
	for i := 0; i < expected; i++ {
		rootContext.Send(a, CreateChildMessage{})
	}
	fut := rootContext.RequestFuture(a, GetChildCountRequest{}, testTimeout)
	response := assertFutureSuccess(fut, t)
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
		context.Spawn(PropsFromProducer(NewBlackHoleActor))
	case GetChildCountMessage2:
		context.Send(msg.ReplyDirectly, true)
		state.replyTo = msg.ReplyAfterStop
	case *Stopped:
		reply := GetChildCountResponse{ChildCount: len(context.Children())}
		context.Send(state.replyTo, reply)
	}
}

func NewCreateChildThenStopActor() Actor {
	return &CreateChildThenStopActor{}
}

func TestActorCanStopChildren(t *testing.T) {
	actor := rootContext.Spawn(PropsFromProducer(NewCreateChildThenStopActor))
	count := 10
	for i := 0; i < count; i++ {
		rootContext.Send(actor, CreateChildMessage{})
	}

	future := NewFuture(system, testTimeout)
	future2 := NewFuture(system, testTimeout)
	rootContext.Send(actor, GetChildCountMessage2{ReplyDirectly: future.PID(), ReplyAfterStop: future2.PID()})

	// wait for the actor to reply to the first responsePID
	assertFutureSuccess(future, t)

	// then send a stop command
	rootContext.Stop(actor)

	// wait for the actor to stop and get the result from the stopped handler
	response := assertFutureSuccess(future2, t)
	// we should have 0 children when the actor is stopped
	assert.Equal(t, 0, response.(GetChildCountResponse).ChildCount)
}

func TestActorReceivesTerminatedFromWatched(t *testing.T) {
	child := rootContext.Spawn(PropsFromFunc(nullReceive))
	future := NewFuture(system, testTimeout)
	var wg sync.WaitGroup
	wg.Add(1)

	var r ReceiveFunc = func(c Context) {
		switch msg := c.Message().(type) {
		case *Started:
			c.Watch(child)
			wg.Done()

		case *Terminated:
			ac := c.(*actorContext)
			if msg.Who.Equal(child) && ac.ensureExtras().watchers.Empty() {
				c.Send(future.PID(), true)
			}
		}
	}

	rootContext.Spawn(PropsFromFunc(r))
	wg.Wait()
	rootContext.Stop(child)

	assertFutureSuccess(future, t)
}

func TestFutureDoesTimeout(t *testing.T) {
	pid := rootContext.Spawn(PropsFromFunc(nullReceive))
	_, err := rootContext.RequestFuture(pid, "", time.Millisecond).Result()
	assert.EqualError(t, err, ErrTimeout.Error())
}

func TestFutureDoesNotTimeout(t *testing.T) {
	var r ReceiveFunc = func(c Context) {
		if _, ok := c.Message().(string); !ok {
			return
		}

		time.Sleep(50 * time.Millisecond)
		c.Respond("foo")
	}
	pid := rootContext.Spawn(PropsFromFunc(r))
	reply, err := rootContext.RequestFuture(pid, "", 2*time.Second).Result()
	assert.NoError(t, err)
	assert.Equal(t, "foo", reply)
}
