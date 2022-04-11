package actor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type (
	DummyMessage   struct{}
	BlackHoleActor struct{}
)

var testTimeout = 1 * time.Second

func (state *BlackHoleActor) Receive(Context) {}

func NewBlackHoleActor() Actor {
	return &BlackHoleActor{}
}

func TestSpawnProducesProcess(t *testing.T) {
	actor := rootContext.Spawn(PropsFromProducer(NewBlackHoleActor))
	defer rootContext.Stop(actor)
	assert.NotNil(t, actor)
}

type EchoRequest struct{}

type EchoResponse struct{}

type EchoActor struct{}

func NewEchoActor() Actor {
	return &EchoActor{}
}

func (*EchoActor) Receive(context Context) {
	switch context.Message().(type) {
	case EchoRequest:
		context.Respond(EchoResponse{})
	}
}

func TestActorCanReplyToMessage(t *testing.T) {
	pid := rootContext.Spawn(PropsFromProducer(NewEchoActor))
	defer rootContext.Stop(pid)
	err := rootContext.RequestFuture(pid, EchoRequest{}, testTimeout).Wait()
	if err != nil {
		assert.Fail(t, "timed out")
		return
	}
}
