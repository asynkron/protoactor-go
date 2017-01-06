package actor

import (
	"sync"
	"testing"
)

type actorWithSupervisor struct {
	wg *sync.WaitGroup
}

func (a *actorWithSupervisor) Receive(ctx Context) {
	switch ctx.Message().(type) {
	case *Started:
		child := ctx.Spawn(FromInstance(&failingChildActor{}))
		child.Tell("Fail!")
	}
}

func (a *actorWithSupervisor) HandleFailure(supervisor Supervisor, child *PID, reason interface{}) {
	a.wg.Done()
}

type failingChildActor struct{}

func (a *failingChildActor) Receive(ctx Context) {
	switch ctx.Message().(type) {
	case string:
		panic("Oh noes!")
	}
}

func TestActorWithOwnSupervisorCanHandleFailure(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	parent := &actorWithSupervisor{wg: wg}
	props := FromInstance(parent)
	Spawn(props)
	wg.Wait()
}
