package router

import (
	"testing"

	"github.com/otherview/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

func TestGroupRouterActor_Receive_AddRoute(t *testing.T) {
	state := new(testRouterState)

	a := groupRouterActor{state: state}

	p1 := actor.NewLocalPID("p1")
	c := new(mockContext)
	c.On("Message").Return(&AddRoutee{p1})
	c.On("Watch", p1).Once()

	state.On("GetRoutees").Return(&actor.PIDSet{})
	state.On("SetRoutees", actor.NewPIDSet(p1)).Once()

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c)
}

func TestGroupRouterActor_Receive_AddRoute_NoDuplicates(t *testing.T) {
	state := new(testRouterState)

	a := groupRouterActor{state: state}

	p1 := actor.NewLocalPID("p1")
	c := new(mockContext)
	c.On("Message").Return(&AddRoutee{p1})

	state.On("GetRoutees").Return(actor.NewPIDSet(p1))

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c)
}

func TestGroupRouterActor_Receive_RemoveRoute(t *testing.T) {
	state := new(testRouterState)

	a := groupRouterActor{state: state}

	p1, _ := spawnMockProcess("p1")
	defer removeMockProcess(p1)

	p2 := actor.NewLocalPID("p2")
	c := new(mockContext)
	c.On("Message").Return(&RemoveRoutee{p1})
	c.On("Unwatch", p1).
		Run(func(args mock.Arguments) {

		}).Once()

	state.On("GetRoutees").Return(actor.NewPIDSet(p1, p2))
	state.On("SetRoutees", actor.NewPIDSet(p2)).Once()

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c)
}

func TestGroupRouterActor_Receive_BroadcastMessage(t *testing.T) {
	state := new(testRouterState)
	a := groupRouterActor{state: state}

	p1 := actor.NewLocalPID("p1")
	p2 := actor.NewLocalPID("p2")

	child := new(mockProcess)
	child.On("SendUserMessage", mock.Anything, mock.Anything).Times(2)

	actor.ProcessRegistry.Add(child, "p1")
	actor.ProcessRegistry.Add(child, "p2")
	defer func() {
		actor.ProcessRegistry.Remove(&actor.PID{Id: "p1"})
		actor.ProcessRegistry.Remove(&actor.PID{Id: "p2"})
	}()

	c := new(mockContext)
	c.On("Message").Return(&BroadcastMessage{"hi"})
	c.On("Sender").Return((*actor.PID)(nil))
	c.On("RequestWithCustomSender").Twice()

	state.On("GetRoutees").Return(actor.NewPIDSet(p1, p2))

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c, child)
}
