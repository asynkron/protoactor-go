package actor

import (
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"log"
	"sync/atomic"
	"time"
)

type receiveFn func(Context)

func (fn receiveFn) Receive(ctx Context) {
	fn(ctx)
}

var nullReceive receiveFn = func(Context) {}

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
