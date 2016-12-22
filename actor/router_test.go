package actor

import (
	"fmt"
	"testing"

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
	child := Spawn(FromInstance(a))

	rs := new(testRouterState)
	rs.On("SetRoutees", []*PID{child})
	rs.On("RouteMessage", "hello", mock.Anything)

	grc := newGroupRouterConfig(child)
	grc.On("CreateRouterState").Return(rs)

	routerPID := Spawn(FromGroupRouter(grc))
	routerPID.Tell("hello")

	w.Wait()

	mock.AssertExpectationsForObjects(t, a, rs)
}

type testGroupRouter struct {
	GroupRouter
	mock.Mock
}

func newGroupRouterConfig(routees ...*PID) *testGroupRouter {
	r := new(testGroupRouter)
	r.Routees = routees
	return r
}

func (m *testGroupRouter) CreateRouterState() RouterState {
	args := m.Called()
	return args.Get(0).(*testRouterState)
}

type testRouterState struct {
	mock.Mock
	routees []*PID
}

func (m *testRouterState) SetRoutees(routees []*PID) {
	m.Called(routees)
	m.routees = routees
}

func (m *testRouterState) RouteMessage(message interface{}, sender *PID) {
	m.Called(message, sender)
	for _, pid := range m.routees {
		pid.Request(message, sender)
	}
}

func (m *testRouterState) GetRoutees() []*PID {
	args := m.Called()
	return args.Get(0).([]*PID)
}
