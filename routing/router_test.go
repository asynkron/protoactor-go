package routing

import (
	"fmt"
	"testing"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

var _ fmt.Formatter

func TestRouterSendsUserMessageToChild(t *testing.T) {
	w := newWaiter(1)

	a := newMockActor()
	a.On("Receive", mock.Anything).
		Run(func(args mock.Arguments) {
			w.Done()
		})
	child := actor.Spawn(actor.FromInstance(a))

	s1 := actor.NewPIDSet(child)

	rs := new(testRouterState)
	rs.On("SetRoutees", s1)
	rs.On("RouteMessage", "hello", mock.Anything)

	grc := newGroupRouterConfig(child)
	grc.On("CreateRouterState").Return(rs)

	routerPID := actor.Spawn(FromGroupRouter(grc))
	routerPID.Tell("hello")

	w.Wait()

	mock.AssertExpectationsForObjects(t, a, rs)
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

func (m *testGroupRouter) CreateRouterState() RouterState {
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
