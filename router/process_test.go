package router

import (
	"fmt"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

var _ fmt.Formatter
var _ time.Time

func TestRouterSendsUserMessageToChild(t *testing.T) {
	child, p := spawnMockProcess("child")
	defer removeMockProcess(child)

	p.On("SendUserMessage", mock.Anything, "hello")
	p.On("SendSystemMessage", mock.Anything, mock.Anything)

	s1 := actor.NewPIDSet(child)

	rs := new(testRouterState)
	rs.On("SetRoutees", s1)
	rs.On("RouteMessage", "hello", mock.Anything)

	grc := newGroupRouterConfig(child)
	grc.On("CreateRouterState").Return(rs)

	routerPID := actor.Spawn(actor.FromSpawnFunc(spawner(grc)))
	routerPID.Tell("hello")
	routerPID.Request("hello", routerPID)

	mock.AssertExpectationsForObjects(t, p, rs)
}

type testGroupRouter struct {
	GroupRouter
	mock.Mock
}

func newGroupRouterConfig(routees ...*actor.PID) *testGroupRouter {
	r := new(testGroupRouter)
	r.Routees = actor.NewPIDSet(routees...)
	return r
}

func (m *testGroupRouter) CreateRouterState() Interface {
	args := m.Called()
	return args.Get(0).(*testRouterState)
}

type testRouterState struct {
	mock.Mock
	routees *actor.PIDSet
}

func (m *testRouterState) SetRoutees(routees *actor.PIDSet) {
	m.Called(routees)
	m.routees = routees
}

func (m *testRouterState) RouteMessage(message interface{}, sender *actor.PID) {
	m.Called(message, sender)
	m.routees.ForEach(func(i int, pid actor.PID) {
		pid.Request(message, sender)
	})
}

func (m *testRouterState) GetRoutees() *actor.PIDSet {
	args := m.Called()
	return args.Get(0).(*actor.PIDSet)
}
