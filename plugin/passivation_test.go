package plugin

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

var system = actor.NewActorSystem()

type SmartActor struct {
	PassivationHolder
}

func (state *SmartActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	}
}

func TestPassivation(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	UnitOfTime := 200 * time.Millisecond
	PassivationDuration := 3 * UnitOfTime
	rootContext := system.Root
	props := actor.
		PropsFromProducer(func() actor.Actor { return &SmartActor{} },
			actor.WithReceiverMiddleware(Use(&PassivationPlugin{Duration: PassivationDuration})))

	pid := rootContext.Spawn(props)
	time.Sleep(UnitOfTime)
	time.Sleep(UnitOfTime)
	{
		_, found := system.ProcessRegistry.GetLocal(pid.Id)
		assert.True(t, found)
	}
	rootContext.Send(pid, "keepalive")
	time.Sleep(UnitOfTime)
	time.Sleep(UnitOfTime)
	{
		_, found := system.ProcessRegistry.GetLocal(pid.Id)
		assert.True(t, found)
	}
	time.Sleep(UnitOfTime)
	time.Sleep(UnitOfTime)
	{
		_, found := system.ProcessRegistry.GetLocal(pid.Id)
		assert.False(t, found)
	}
}
