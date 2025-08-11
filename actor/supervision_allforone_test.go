package actor

import (
	"testing"
	"time"
)

// childActor panics when it receives the string "fail".
type childActor struct{}

func (a *childActor) Receive(ctx Context) {
	if msg, ok := ctx.Message().(string); ok {
		if msg == "fail" {
			panic("boom")
		}
	}
}

// parentActor spawns two children and reports their PIDs through the provided channels.
type parentActor struct {
	child1Props *Props
	child2Props *Props
	ch1         chan *PID
	ch2         chan *PID
}

func (p *parentActor) Receive(ctx Context) {
	switch ctx.Message().(type) {
	case *Started:
		c1 := ctx.Spawn(p.child1Props)
		c2 := ctx.Spawn(p.child2Props)
		p.ch1 <- c1
		p.ch2 <- c2
	}
}

// expectNoMessageWithin ensures no message is received within the given duration.
func expectNoMessageWithin(t *testing.T, e *Expector, d time.Duration) {
	t.Helper()
	select {
	case msg := <-e.C:
		t.Fatalf("expected no message, got %T:%v", msg, msg)
	case <-time.After(d):
	}
}

func TestAllForOneStrategyResume(t *testing.T) {
	// observers to watch messages received by each child
	m1, e1 := NewObserver()
	m2, e2 := NewObserver()

	child1Props := PropsFromProducer(func() Actor { return &childActor{} }, WithReceiverMiddleware(m1))
	child2Props := PropsFromProducer(func() Actor { return &childActor{} }, WithReceiverMiddleware(m2))

	ch1 := make(chan *PID, 1)
	ch2 := make(chan *PID, 1)

	decider := func(reason interface{}) Directive { return ResumeDirective }
	parentProps := PropsFromProducer(func() Actor {
		return &parentActor{child1Props: child1Props, child2Props: child2Props, ch1: ch1, ch2: ch2}
	}, WithSupervisor(NewAllForOneStrategy(1, time.Second, decider)))

	parentPID := rootContext.Spawn(parentProps)
	child1 := <-ch1
	child2 := <-ch2

	e1.ExpectMsg(startedMessage, t)
	e2.ExpectMsg(startedMessage, t)

	rootContext.Send(child1, "fail")
	e1.ExpectMsg("fail", t)

	// allow the supervisor to process the failure
	time.Sleep(10 * time.Millisecond)

	rootContext.Send(child1, "ping1")
	rootContext.Send(child2, "ping2")

	e1.ExpectMsg("ping1", t)
	e2.ExpectMsg("ping2", t)

	// no additional lifecycle messages should appear
	expectNoMessageWithin(t, e1, 50*time.Millisecond)
	expectNoMessageWithin(t, e2, 50*time.Millisecond)

	_ = rootContext.StopFuture(parentPID).Wait()
}

func TestAllForOneStrategyRestart(t *testing.T) {
	m1, e1 := NewObserver()
	m2, e2 := NewObserver()

	child1Props := PropsFromProducer(func() Actor { return &childActor{} }, WithReceiverMiddleware(m1))
	child2Props := PropsFromProducer(func() Actor { return &childActor{} }, WithReceiverMiddleware(m2))

	ch1 := make(chan *PID, 1)
	ch2 := make(chan *PID, 1)

	decider := func(reason interface{}) Directive { return RestartDirective }
	parentProps := PropsFromProducer(func() Actor {
		return &parentActor{child1Props: child1Props, child2Props: child2Props, ch1: ch1, ch2: ch2}
	}, WithSupervisor(NewAllForOneStrategy(1, time.Second, decider)))

	parentPID := rootContext.Spawn(parentProps)
	child1 := <-ch1
	child2 := <-ch2

	e1.ExpectMsg(startedMessage, t)
	e2.ExpectMsg(startedMessage, t)

	rootContext.Send(child1, "fail")
	e1.ExpectMsg("fail", t)

	e1.ExpectMsg(restartingMessage, t)
	e1.ExpectMsg(startedMessage, t)
	e2.ExpectMsg(restartingMessage, t)
	e2.ExpectMsg(startedMessage, t)

	rootContext.Send(child1, "ping1")
	rootContext.Send(child2, "ping2")

	e1.ExpectMsg("ping1", t)
	e2.ExpectMsg("ping2", t)

	_ = rootContext.StopFuture(parentPID).Wait()
}

func TestAllForOneStrategyStop(t *testing.T) {
	m1, e1 := NewObserver()
	m2, e2 := NewObserver()

	child1Props := PropsFromProducer(func() Actor { return &childActor{} }, WithReceiverMiddleware(m1))
	child2Props := PropsFromProducer(func() Actor { return &childActor{} }, WithReceiverMiddleware(m2))

	ch1 := make(chan *PID, 1)
	ch2 := make(chan *PID, 1)

	decider := func(reason interface{}) Directive { return StopDirective }
	parentProps := PropsFromProducer(func() Actor {
		return &parentActor{child1Props: child1Props, child2Props: child2Props, ch1: ch1, ch2: ch2}
	}, WithSupervisor(NewAllForOneStrategy(1, time.Second, decider)))

	parentPID := rootContext.Spawn(parentProps)
	child1 := <-ch1
	child2 := <-ch2

	e1.ExpectMsg(startedMessage, t)
	e2.ExpectMsg(startedMessage, t)

	rootContext.Send(child1, "fail")
	e1.ExpectMsg("fail", t)

	e1.ExpectMsg(stoppingMessage, t)
	e2.ExpectMsg(stoppingMessage, t)

	e1.ExpectMsg(stoppedMessage, t)
	e2.ExpectMsg(stoppedMessage, t)

	rootContext.Send(child1, "ping1")
	rootContext.Send(child2, "ping2")

	expectNoMessageWithin(t, e1, 50*time.Millisecond)
	expectNoMessageWithin(t, e2, 50*time.Millisecond)

	_ = rootContext.StopFuture(parentPID).Wait()
}
