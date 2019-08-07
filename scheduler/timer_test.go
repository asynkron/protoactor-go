package scheduler

import (
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestNewTimerScheduler(t *testing.T) {
	newActor := func(t *testing.T, n int) (pid *actor.PID, ch chan struct{}) {
		ch = make(chan struct{}, n)
		props := actor.PropsFromFunc(func(c actor.Context) {
			switch c.Message().(type) {
			case string:
				select {
				case ch <- struct{}{}:
				default:
					t.Errorf("exceeeded expected count %d", n)
				}

			}
		})
		return actor.EmptyRootContext.Spawn(props), ch
	}

	// check verifies the number of times ch receives a message matches exp
	// and executes once more to ensure no further messages are received
	check := func(t *testing.T, ch chan struct{}, cancel CancelFunc, exp int) {
		got := 0
		for i := 0; i < exp+1; i++ {
			select {
			case <-ch:
				got++
				if got == exp {
					cancel()
				}
			case <-time.After(3 * time.Millisecond):
				if got != exp {
					assert.Fail(t, "failed to receive message")
				}
			}
		}
		cancel()
		assert.Equal(t, exp, got)
	}

	t.Run("does", func(t *testing.T) {
		t.Run("send once", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 1)
			tok := s.SendOnce(1*time.Millisecond, pid, "hello")

			check(t, ch, tok, 1)
		})

		t.Run("send repeatedly", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 5)
			tok := s.SendRepeatedly(1*time.Millisecond, 1*time.Millisecond, pid, "hello")
			check(t, ch, tok, 5)
		})

		t.Run("request once", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 1)
			tok := s.RequestOnce(1*time.Millisecond, pid, "hello")

			check(t, ch, tok, 1)
		})

		t.Run("request repeatedly", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 5)
			tok := s.RequestRepeatedly(1*time.Millisecond, 1*time.Millisecond, pid, "hello")
			check(t, ch, tok, 5)
		})
	})

	t.Run("does not", func(t *testing.T) {
		t.Run("send once", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 1)
			cancel := s.SendOnce(1*time.Millisecond, pid, "hello")
			cancel()
			check(t, ch, cancel, 0)
		})

		t.Run("send repeatedly", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 5)
			cancel := s.SendRepeatedly(1*time.Millisecond, 1*time.Millisecond, pid, "hello")
			cancel()
			check(t, ch, cancel, 0)
		})

		t.Run("request once", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 1)
			cancel := s.RequestOnce(1*time.Millisecond, pid, "hello")
			cancel()
			check(t, ch, cancel, 0)
		})

		t.Run("request repeatedly", func(t *testing.T) {
			s := NewTimerScheduler()
			pid, ch := newActor(t, 5)
			cancel := s.RequestRepeatedly(1*time.Millisecond, 1*time.Millisecond, pid, "hello")
			cancel()
			check(t, ch, cancel, 0)
		})
	})

}
