package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeadLetterAfterStop(t *testing.T) {
	a := rootContext.Spawn(PropsFromProducer(NewBlackHoleActor))
	done := false
	sub := system.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if deadLetter.PID == a {
				done = true
			}
		}
	})
	defer system.EventStream.Unsubscribe(sub)

	_ = rootContext.StopFuture(a).Wait()

	rootContext.Send(a, "hello")

	assert.True(t, done)
}

func TestDeadLetterWatchRespondsWithTerminate(t *testing.T) {
	// create an actor
	pid := rootContext.Spawn(PropsFromProducer(NewBlackHoleActor))
	// stop id
	_ = rootContext.StopFuture(pid).Wait()
	f := NewFuture(system, testTimeout)
	// send a watch message, from our future
	pid.sendSystemMessage(system, &Watch{Watcher: f.PID()})
	assertFutureSuccess(f, t)
}
