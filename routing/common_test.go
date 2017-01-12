package routing

import (
	"io/ioutil"
	"log"
	"sync/atomic"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/mock"
)

var nullReceive actor.Receive = func(actor.Context) {}

func init() {
	// discard all logging in tests
	log.SetOutput(ioutil.Discard)
}

type inlineDispatcher struct{}

func (inlineDispatcher) Schedule(runner actor.MailboxRunner) {
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

func (m *mockActor) Receive(context actor.Context) {
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

func (m *mockContext) Watch(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) Unwatch(pid *actor.PID) {
	m.Called(pid)
}

func (m *mockContext) Message() interface{} {
	args := m.Called()
	return args.Get(0)
}

func (m *mockContext) SetReceiveTimeout(d time.Duration) {
	m.Called(d)
}
func (m *mockContext) ReceiveTimeout() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *mockContext) Sender() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Become(r actor.Receive) {
	m.Called(r)
}

func (m *mockContext) BecomeStacked(r actor.Receive) {
	m.Called(r)
}

func (m *mockContext) UnbecomeStacked() {
	m.Called()
}

func (m *mockContext) Self() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Parent() *actor.PID {
	args := m.Called()
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Spawn(p actor.Props) *actor.PID {
	args := m.Called(p)
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) SpawnNamed(p actor.Props, name string) *actor.PID {
	args := m.Called(p, name)
	return args.Get(0).(*actor.PID)
}

func (m *mockContext) Children() []*actor.PID {
	args := m.Called()
	return args.Get(0).([]*actor.PID)
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

func (m *mockContext) Actor() actor.Actor {
	args := m.Called()
	return args.Get(0).(actor.Actor)
}

type mockProcess struct {
	mock.Mock
}

func (m *mockProcess) SendUserMessage(pid *actor.PID, message interface{}, sender *actor.PID) {
	m.Called(pid, message, sender)
}
func (m *mockProcess) SendSystemMessage(pid *actor.PID, message actor.SystemMessage) {
	m.Called(pid, message)
}
func (m *mockProcess) Stop(pid *actor.PID) {
	m.Called(pid)
}
func (m *mockProcess) Watch(pid *actor.PID) {
	m.Called(pid)
}
func (m *mockProcess) Unwatch(pid *actor.PID) {
	m.Called(pid)
}
