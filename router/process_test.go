package router

import (
	"fmt"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

var (
	_ fmt.Formatter
	_ time.Time
)

// TODO fix this
func __TestRouterSendsUserMessageToChild(t *testing.T) {
	child, p := spawnMockProcess("child")
	defer removeMockProcess(child)

	p.On("SendUserMessage", mock.Anything, mock.MatchedBy(func(env interface{}) bool {
		_, msg, _ := actor.UnwrapEnvelope(env)
		return msg.(string) == "hello"
	}))
	p.On("SendSystemMessage", mock.Anything, mock.Anything)

	s1 := actor.NewPIDSet(child)

	rs := new(testRouterState)
	//	rs.On("SetSender",)
	rs.On("SetRoutees", s1)
	rs.On("RouteMessage", mock.MatchedBy(func(env interface{}) bool {
		_, msg, _ := actor.UnwrapEnvelope(env)
		return msg.(string) == "hello"
	}), mock.Anything)

	grc := newGroupRouterConfig(child)
	grc.On("CreateRouterState").Return(rs)

	routerPID := system.Root.Spawn((&actor.Props{}).Configure(actor.WithSpawnFunc(spawner(grc))))
	system.Root.Send(routerPID, "hello")
	system.Root.RequestWithCustomSender(routerPID, "hello", routerPID)

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

func (m *testGroupRouter) CreateRouterState() State {
	args := m.Called()
	return args.Get(0).(*testRouterState)
}

type testRouterState struct {
	mock.Mock
	routees *actor.PIDSet
	sender  actor.SenderContext
}

func (m *testRouterState) SetSender(sender actor.SenderContext) {
	m.Called(sender)
	m.sender = sender
}

func (m *testRouterState) SetRoutees(routees *actor.PIDSet) {
	m.Called(routees)
	m.routees = routees
}

func (m *testRouterState) RouteMessage(message interface{}) {
	m.Called(message)
	m.routees.ForEach(func(i int, pid *actor.PID) {
		system.Root.Send(pid, message)
	})
}

func (m *testRouterState) GetRoutees() *actor.PIDSet {
	args := m.Called()
	return args.Get(0).(*actor.PIDSet)
}
