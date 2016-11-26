package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeadLetterAfterStop(t *testing.T) {
	actor := Spawn(FromProducer(NewBlackHoleActor))
	done := false
	sub := EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetter); ok {
			if deadLetter.PID == actor {
				done = true
			}
		}
	})
	defer EventStream.Unsubscribe(sub)

	stop := actor.StopFuture()
	stop.Wait()

	actor.Tell("hello")

	assert.True(t, done)
}
