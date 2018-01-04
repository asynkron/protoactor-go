package actor

import (
	"math/rand"
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
		ctx.Tell(child, "Fail!")
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
	if actual != expected {
		t.Errorf("Expected %T:%v, got %T:%v", expected, expected, actual, actual)
	}
}

func (e *Expector) ExpectNoMsg(t *testing.T) {
	select {
	case actual := <-e.C:
		t.Errorf("Expected no message got %T:%v", actual, actual)
	case <-time.After(time.Second * 1):
		// pass
	}
}

func TestActorStopsAfterXRestarts(t *testing.T) {
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

type templateStruct struct {
	num int
}

func (s *templateStruct) Receive(c Context) {
	if msg, ok := c.Message().(int); ok {
		switch {
		case msg == 0:
			c.Respond(s.num)
		case msg > 0:
			s.num = msg
		case msg < 0:
			// Restart by default.
			panic("negative num")
		}
	}
}

type templateMap map[int]struct{}

func (m templateMap) Receive(c Context) {
	if msg, ok := c.Message().(int); ok {
		switch {
		case msg == 0:
			c.Respond(m)
		case msg > 0:
			m[msg] = struct{}{}
		case msg < 0:
			// Restart by default.
			panic("negative num")
		}
	}
}

func TestActorResetFromTemplateAfterRestart(t *testing.T) {
	{
		old := &templateStruct{num: 1}
		pid := Spawn(FromTemplate(old))
		num := rand.Intn(1024) + 1                 // random positive number
		pid.Tell(num)                              // modify the actor's state
		fut := pid.RequestFuture(0, 1*time.Second) // query current state
		res, _ := fut.Result()
		if res.(int) != num {
			t.Errorf("Expected updated actor's state (%d), got %d", num, res)
		}
		pid.Tell(-1)                              // panic
		fut = pid.RequestFuture(0, 1*time.Second) // query current state
		res, _ = fut.Result()
		if res.(int) != 1 {
			t.Errorf("Expected original actor's state (%d), got %d", old.num, res)
		}
	}
	{
		old := templateMap{1: struct{}{}}
		pid := Spawn(FromTemplate(old))
		num := rand.Intn(1024) + 1                 // random positive number
		pid.Tell(num)                              // modify the actor's state
		fut := pid.RequestFuture(0, 1*time.Second) // query current state
		res, _ := fut.Result()
		if _, found := res.(templateMap)[num]; !found || len(res.(templateMap)) != len(old)+1 {
			t.Errorf("Expected updated actor's state (%v + %d:{})), got %v", old, num, res)
		}
		pid.Tell(-1)                              // panic
		fut = pid.RequestFuture(0, 1*time.Second) // query current state
		res, _ = fut.Result()
		if !reflect.DeepEqual(res, old) {
			t.Errorf("Expected original actor's state (%v), got %v", old, res)
		}
	}
}
