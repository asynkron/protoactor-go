package scheduler

import (
	"runtime"
	"sync/atomic"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type CancelFunc func()

type Stopper interface {
	Stop()
}

const (
	stateInit = iota
	stateReady
	stateDone
)

func startTimer(delay, interval time.Duration, fn func()) CancelFunc {
	var t *time.Timer
	var state int32
	t = time.AfterFunc(delay, func() {
		state := atomic.LoadInt32(&state)
		for state == stateInit {
			runtime.Gosched()
			state = atomic.LoadInt32(&state)
		}

		if state == stateDone {
			return
		}

		fn()
		t.Reset(interval)
	})

	// ensures t != nil and is required to avoid data race in
	// AfterFunc calling t.Reset
	atomic.StoreInt32(&state, stateReady)

	return func() {
		if atomic.SwapInt32(&state, stateDone) != stateDone {
			t.Stop()
		}
	}
}

// A scheduler utilizing timers to send messages in the future and at regular intervals.
type TimerScheduler struct {
	ctx actor.SenderContext
}

type timerOptionFunc func(*TimerScheduler)

// WithContext configures the scheduler to use ctx rather than the default,
// EmptyRootContext.
func WithContext(ctx actor.SenderContext) timerOptionFunc {
	return func(s *TimerScheduler) {
		s.ctx = ctx
	}
}

// NewTimerScheduler creates a new scheduler using the EmptyRootContext.
// Additional options may be specified to override the default behavior.
func NewTimerScheduler(opts ...timerOptionFunc) *TimerScheduler {
	s := &TimerScheduler{ctx: actor.EmptyRootContext}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SendOnce waits for the duration to elapse and then calls actor.SenderContext.Send to forward the message to pid.
func (s *TimerScheduler) SendOnce(delay time.Duration, pid *actor.PID, message interface{}) CancelFunc {
	t := time.AfterFunc(delay, func() {
		s.ctx.Send(pid, message)
	})

	return func() { t.Stop() }
}

// SendRepeatedly waits for the initial duration to elapse and then calls Send to forward the message to pid
// repeatedly for each interval.
func (s *TimerScheduler) SendRepeatedly(initial, interval time.Duration, pid *actor.PID, message interface{}) CancelFunc {
	return startTimer(initial, interval, func() {
		s.ctx.Send(pid, message)
	})
}

// RequestOnce waits for the duration to elapse and then calls actor.SenderContext.Request to forward the message to
// pid.
func (s *TimerScheduler) RequestOnce(delay time.Duration, pid *actor.PID, message interface{}) CancelFunc {
	t := time.AfterFunc(delay, func() {
		s.ctx.Request(pid, message)
	})

	return func() { t.Stop() }
}

// RequestRepeatedly waits for the initial duration to elapse and then calls Request to forward the message to pid
// repeatedly for each interval.
func (s *TimerScheduler) RequestRepeatedly(delay, interval time.Duration, pid *actor.PID, message interface{}) CancelFunc {
	return startTimer(delay, interval, func() {
		s.ctx.Request(pid, message)
	})
}
