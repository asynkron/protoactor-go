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
		child := ctx.Spawn(PropsFromProducer(func() Actor { return &failingChildActor{} }))
		ctx.Send(child, "Fail!")
	}
}

func (a *actorWithSupervisor) HandleFailure(*ActorSystem, Supervisor, *PID, *RestartStatistics, interface{}, interface{}) {
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
	props := PropsFromProducer(func() Actor { return &actorWithSupervisor{wg: wg} })
	rootContext.Spawn(props)
	wg.Wait()
}

func NewObserver() (func(ReceiverFunc) ReceiverFunc, *Expector) {
	c := make(chan interface{})
	e := &Expector{C: c}
	f := func(next ReceiverFunc) ReceiverFunc {
		fn := func(context ReceiverContext, env *MessageEnvelope) {
			message := env.Message
			c <- message
			next(context, env)
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
		// pass
	}
}

func TestActorStopsAfterXRestarts(t *testing.T) {
	m, e := NewObserver()
	props := PropsFromProducer(func() Actor { return &failingChildActor{} }, WithReceiverMiddleware(m))
	child := rootContext.Spawn(props)
	fail := "fail!"

	e.ExpectMsg(startedMessage, t)

	// root supervisor allows 10 restarts
	for i := 0; i < 10; i++ {
		rootContext.Send(child, fail)
		e.ExpectMsg(fail, t)
		e.ExpectMsg(restartingMessage, t)
		e.ExpectMsg(startedMessage, t)
	}
	rootContext.Send(child, fail)
	e.ExpectMsg(fail, t)
	// the 11th time should cause a termination
	e.ExpectMsg(stoppingMessage, t)
}
