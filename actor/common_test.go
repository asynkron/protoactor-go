package actor

import (
	"io/ioutil"
	"log"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/mock"
)

var nullReceive Receive = func(Context) {}

func init() {
	// discard all logging in tests
	log.SetOutput(ioutil.Discard)
}

type inlineDispatcher struct{}

func (inlineDispatcher) Schedule(runner MailboxRunner) {
	runner()
}

func (inlineDispatcher) Throughput() int {
	return 1
}

const defaultTimeout = 10 * time.Millisecond

type waiter struct {
	c  int32
	ch chan struct{}
}

func newWaiter(c int32) *waiter {
	return &waiter{c: c, ch: make(chan struct{})}
}

func (w *waiter) Add(c int32) {
	v := atomic.AddInt32(&w.c, c)
	if v == 0 {
		w.ch <- struct{}{}
	} else if v < 0 {
		panic("<0")
	}
}

func (w *waiter) Done() {
	w.Add(-1)
}

func (w *waiter) Wait() bool {
	return w.WaitTimeout(defaultTimeout)
}

func (w *waiter) WaitTimeout(t time.Duration) bool {
	select {
	case <-w.ch:
		return true
	case <-time.After(t):
		return false
	}
}

type mockActor struct {
	mock.Mock
}

func (m *mockActor) Receive(context Context) {
	m.Called(context)
}

func newMockActor() *mockActor {
	a := new(mockActor)
	a.On("Receive", mock.Anything).Once() // Started
	return a
}

type mockContext struct {
	mock.Mock
}

func (m *mockContext) Watch(pid *PID) {
	m.Called(pid)
}

func (m *mockContext) Unwatch(pid *PID) {
	m.Called(pid)
}

func (m *mockContext) Message() interface{} {
	args := m.Called()
	return args.Get(0)
}

func (m *mockContext) Sender() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) Become(r Receive) {
	m.Called(r)
}

func (m *mockContext) BecomeStacked(r Receive) {
	m.Called(r)
}

func (m *mockContext) UnbecomeStacked() {
	m.Called()
}

func (m *mockContext) Self() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) Parent() *PID {
	args := m.Called()
	return args.Get(0).(*PID)
}

func (m *mockContext) Spawn(p Props) *PID {
	args := m.Called(p)
	return args.Get(0).(*PID)
}

func (m *mockContext) SpawnNamed(p Props, name string) *PID {
	args := m.Called(p, name)
	return args.Get(0).(*PID)
}

func (m *mockContext) Children() []*PID {
	args := m.Called()
	return args.Get(0).([]*PID)
}

func (m *mockContext) Next() {
	m.Called()
}

func (m *mockContext) Receive(i interface{}) {
	m.Called(i)
}

func (m *mockContext) Stash() {
	m.Called()
}

func (m *mockContext) Respond(response interface{}) {
	m.Called(response)
}

func (m *mockContext) Actor() Actor {
	args := m.Called()
	return args.Get(0).(Actor)
}

type mockActorRef struct {
	mock.Mock
}

func (m *mockActorRef) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	m.Called(pid, message, sender)
}
func (m *mockActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	m.Called(pid, message)
}
func (m *mockActorRef) Stop(pid *PID) {
	m.Called(pid)
}
func (m *mockActorRef) Watch(pid *PID) {
	m.Called(pid)
}
func (m *mockActorRef) Unwatch(pid *PID) {
	m.Called(pid)
}
