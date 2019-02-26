package plugin

import (
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

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

	UnitOfTime := time.Duration(200 * time.Millisecond)
	PassivationDuration := time.Duration(3 * UnitOfTime)
	rootContext := actor.EmptyRootContext
	props := actor.
		PropsFromProducer(func() actor.Actor { return &SmartActor{} }).
		WithReceiverMiddleware(Use(&PassivationPlugin{Duration: PassivationDuration}))

	pid := rootContext.Spawn(props)
	time.Sleep(UnitOfTime)
	time.Sleep(UnitOfTime)
	{
		_, found := actor.ProcessRegistry.LocalPIDs.Get(pid.Id)
		assert.True(t, found)
	}
	rootContext.Send(pid, "keepalive")
	time.Sleep(UnitOfTime)
	time.Sleep(UnitOfTime)
	{
		_, found := actor.ProcessRegistry.LocalPIDs.Get(pid.Id)
		assert.True(t, found)
	}
	time.Sleep(UnitOfTime)
	time.Sleep(UnitOfTime)
	{
		_, found := actor.ProcessRegistry.LocalPIDs.Get(pid.Id)
		assert.False(t, found)
	}
}
