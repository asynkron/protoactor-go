package actor

import (
	"reflect"
	"sync"
	"testing"
	"time"
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

func (a *actorWithSupervisor) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {
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

func NewObserver() (func(ActorFunc) ActorFunc, *Expector) {
	c := make(chan interface{})
	e := &Expector{C: c}
	f := func(next ActorFunc) ActorFunc {
		fn := func(context Context) {
			message := context.Message()
			c <- message
			next(context)
		}

		return fn
	}
	return f, e
}

type Expector struct {
	C <-chan interface{}
}

func (e *Expector) ExpectMsg(expected interface{}, t *testing.T) {
	actual := <-e.C
	if actual == expected {

	} else {

		at := reflect.TypeOf(actual)
		et := reflect.TypeOf(expected)
		t.Errorf("Expected %v:%v, got %v:%v", et, expected, at, actual)
	}
}

func (e *Expector) ExpectNoMsg(t *testing.T) {

	select {
	case actual := <-e.C:
		at := reflect.TypeOf(actual)
		t.Errorf("Expected no message got %v:%v", at, actual)
	case <-time.After(time.Second * 1):
		//pass
	}
}

func TestActorStopsAfterXRestars(t *testing.T) {
	m, e := NewObserver()
	props := FromInstance(&failingChildActor{}).WithMiddleware(m)
	child := Spawn(props)
	fail := "fail!"

	e.ExpectMsg(startedMessage, t)

	//root supervisor allows 10 restarts
	for i := 0; i < 10; i++ {
		child.Tell(fail)
		e.ExpectMsg(fail, t)
		e.ExpectMsg(restartingMessage, t)
		e.ExpectMsg(startedMessage, t)
	}
	child.Tell(fail)
	e.ExpectMsg(fail, t)
	//the 11th time should cause a termination
	e.ExpectMsg(stoppingMessage, t)
}
