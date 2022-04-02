package router

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

func TestPoolRouterActor_Receive_AddRoute(t *testing.T) {
	state := new(testRouterState)

	a := poolRouterActor{state: state}

	p1 := system.NewLocalPID("p1")
	c := new(mockContext)
	c.On("Message").Return(&AddRoutee{PID: p1})
	c.On("Watch", p1).Once()

	state.On("GetRoutees").Return(&actor.PIDSet{})
	state.On("SetRoutees", actor.NewPIDSet(p1)).Once()

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c)
}

func TestPoolRouterActor_Receive_AddRoute_NoDuplicates(t *testing.T) {
	state := new(testRouterState)

	a := poolRouterActor{state: state}

	p1 := system.NewLocalPID("p1")
	c := new(mockContext)
	c.On("Message").Return(&AddRoutee{PID: p1})

	state.On("GetRoutees").Return(actor.NewPIDSet(p1))

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c)
}

func TestPoolRouterActor_Receive_RemoveRoute(t *testing.T) {
	state := new(testRouterState)

	a := poolRouterActor{state: state}

	p1, pr1 := spawnMockProcess("p1")
	defer removeMockProcess(p1)
	pr1.On("SendUserMessage", p1, &actor.PoisonPill{}).Once()

	p2 := system.NewLocalPID("p2")
	c := new(mockContext)
	c.On("Message").Return(&RemoveRoutee{PID: p1})
	c.On("Unwatch", p1).Once()

	c.On("Send")

	state.On("GetRoutees").Return(actor.NewPIDSet(p1, p2))
	state.On("SetRoutees", actor.NewPIDSet(p2)).Once()

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c)
}

func TestPoolRouterActor_Receive_BroadcastMessage(t *testing.T) {
	state := new(testRouterState)
	a := poolRouterActor{state: state}

	p1 := system.NewLocalPID("p1")
	p2 := system.NewLocalPID("p2")

	child := new(mockProcess)
	child.On("SendUserMessage", mock.Anything, mock.Anything).Times(2)

	system.ProcessRegistry.Add(child, "p1")
	system.ProcessRegistry.Add(child, "p2")
	defer func() {
		system.ProcessRegistry.Remove(&actor.PID{Id: "p1"})
		system.ProcessRegistry.Remove(&actor.PID{Id: "p2"})
	}()

	c := new(mockContext)
	c.On("Message").Return(&BroadcastMessage{"hi"})
	c.On("Sender").Return((*actor.PID)(nil))
	c.On("RequestWithCustomSender").Twice()

	state.On("GetRoutees").Return(actor.NewPIDSet(p1, p2))

	a.Receive(c)
	mock.AssertExpectationsForObjects(t, state, c, child)
}
