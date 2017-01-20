package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeadLetterAfterStop(t *testing.T) {
	actor := Spawn(FromProducer(NewBlackHoleActor))
	done := false
	sub := EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if deadLetter.PID == actor {
				done = true
			}
		}
	})
	defer EventStream.Unsubscribe(sub)

	actor.
		StopFuture().
		Wait()

	actor.Tell("hello")

	assert.True(t, done)
}
