package actor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequestToStoppedActorReturnsDeadLetterError(t *testing.T) {
	pid := rootContext.Spawn(PropsFromProducer(NewBlackHoleActor))
	_ = rootContext.StopFuture(pid).Wait()

	future := rootContext.RequestFuture(pid, "hello", testTimeout)
	res, err := future.Result()
	assert.Nil(t, res)
	assert.Equal(t, ErrDeadLetter, err)
}

func TestDeadLetterResponseContainsTargetPID(t *testing.T) {
	pid := rootContext.Spawn(PropsFromProducer(NewBlackHoleActor))
	_ = rootContext.StopFuture(pid).Wait()

	ch := make(chan *DeadLetterResponse, 1)
	sender := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		if msg, ok := ctx.Message().(*DeadLetterResponse); ok {
			ch <- msg
		}
	}))

	rootContext.RequestWithCustomSender(pid, "hello", sender)

	select {
	case msg := <-ch:
		assert.Equal(t, pid, msg.Target)
	case <-time.After(testTimeout):
		t.Fatalf("did not receive dead letter response")
	}

	rootContext.Stop(sender)
}
