package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

	a.AssertExpectations(t)
	rs.AssertExpectations(t)
}

func TestRouterProcessesBroadcastMessage(t *testing.T) {
	w := newWaiter(2)

	mc := func() *mockActor {
		a := newMockActor()
		a.On("Receive", mock.Anything).
			Run(func(args mock.Arguments) {
				c := args.Get(0).(Context)
				assert.Equal(t, "hello", c.Message())
				w.Done()
			}).
			Once()
		return a
	}

	a1 := mc()
	a2 := mc()

	p1 := Spawn(FromInstance(a1))
	p2 := Spawn(FromInstance(a2))

	routees := []*PID{p1, p2}

	rs := new(testRouterState)
	rs.On("SetRoutees", routees).Once()
	rs.On("GetRoutees").Return(routees).Once()

	grc := newGroupRouterConfig(routees...)
	grc.On("CreateRouterState").Return(rs)

	routerPID := Spawn(FromGroupRouter(grc))
	routerPID.Tell(&RouterBroadcastMessage{"hello"})

	assert.True(t, w.Wait())

	a1.AssertExpectations(t)
	a2.AssertExpectations(t)
	rs.AssertExpectations(t)
}

func TestRouterProcessesAddRouteeMessage(t *testing.T) {
	p1 := NewLocalPID("p1")
	p2 := NewLocalPID("p2")

	routees := []*PID{p1}

	w := newWaiter(1)

	rs := new(testRouterState)
	rs.On("SetRoutees", routees).Once()
	rs.On("GetRoutees").Return(routees).Once()
	rs.On("SetRoutees", []*PID{p1, p2}).
		Run(func(mock.Arguments) {
			w.Done()
		}).
		Once()

	grc := newGroupRouterConfig(routees...)
	grc.On("CreateRouterState").Return(rs)

	routerPID := Spawn(FromGroupRouter(grc))

	routerPID.Tell(&RouterAddRoutee{p2})

	w.Wait()

	rs.AssertExpectations(t)
}

func TestRouterProcessesRemoveRouteeMessage(t *testing.T) {
	p1 := NewLocalPID("p1")
	p2 := NewLocalPID("p2")

	routees := []*PID{p1, p2}

	w := newWaiter(1)

	rs := new(testRouterState)
	rs.On("SetRoutees", routees).Once()
	rs.On("GetRoutees").Return(routees).Once()
	rs.On("SetRoutees", []*PID{p1}).
		Run(func(mock.Arguments) {
			w.Done()
		}).
		Once()

	grc := newGroupRouterConfig(routees...)
	grc.On("CreateRouterState").Return(rs)

	routerPID := Spawn(FromGroupRouter(grc))

	routerPID.Tell(&RouterRemoveRoutee{p2})

	w.Wait()

	rs.AssertExpectations(t)
}

func TestRouterProcessesGetRouteeMessage(t *testing.T) {
	p1 := NewLocalPID("p1")
	p2 := NewLocalPID("p2")

	routees := []*PID{p1, p2}

	rs := new(testRouterState)
	rs.On("SetRoutees", routees).Once()
	rs.On("GetRoutees").Return(routees).Once()

	grc := newGroupRouterConfig(routees...)
	grc.On("CreateRouterState").Return(rs)

	routerPID := Spawn(FromGroupRouter(grc))

	v, err := routerPID.RequestFuture(&RouterGetRoutees{}, defaultTimeout).Result()
	assert.NoError(t, err)
	assert.IsType(t, &RouterRoutees{}, v)

	res := v.(*RouterRoutees)
	assert.Equal(t, routees, res.PIDs)

	rs.AssertExpectations(t)
}

type testRouterStateMock struct {
	mock.Mock
}

func (m *testRouterStateMock) RouteMessage(message interface{}, sender *PID) {
	m.Called(message, sender)
}

func (m *testRouterStateMock) SetRoutees(routees []*PID) {
	m.Called(routees)
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
