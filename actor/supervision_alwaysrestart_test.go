package actor

import (
	"sync/atomic"
	"testing"
	"time"
)

type alwaysRestartParent struct {
	failProps    *Props
	siblingProps *Props
	pidCh        chan childPair
}

type childPair struct {
	failPID    *PID
	siblingPID *PID
}

func (p *alwaysRestartParent) Receive(ctx Context) {
	switch ctx.Message().(type) {
	case *Started:
		failPID := ctx.Spawn(p.failProps)
		siblingPID := ctx.Spawn(p.siblingProps)
		p.pidCh <- childPair{failPID: failPID, siblingPID: siblingPID}
	}
}

type restartingChild struct {
	starts    int32
	startedCh chan struct{}
}

func (c *restartingChild) Receive(ctx Context) {
	switch ctx.Message().(type) {
	case *Started:
		atomic.AddInt32(&c.starts, 1)
		if c.startedCh != nil {
			c.startedCh <- struct{}{}
		}
	case string:
		panic("boom")
	}
}

type siblingChild struct {
	starts    int32
	startedCh chan struct{}
}

func (c *siblingChild) Receive(ctx Context) {
	if _, ok := ctx.Message().(*Started); ok {
		atomic.AddInt32(&c.starts, 1)
		if c.startedCh != nil {
			c.startedCh <- struct{}{}
		}
	}
}

func TestAlwaysRestartStrategy_RestartsFailingChild(t *testing.T) {
	failStart := make(chan struct{}, 10)
	siblingStart := make(chan struct{}, 1)

	failing := &restartingChild{startedCh: failStart}
	sibling := &siblingChild{startedCh: siblingStart}

	failProps := PropsFromProducer(func() Actor { return failing })
	siblingProps := PropsFromProducer(func() Actor { return sibling })

	pidCh := make(chan childPair, 1)
	parentProps := PropsFromProducer(func() Actor {
		return &alwaysRestartParent{failProps: failProps, siblingProps: siblingProps, pidCh: pidCh}
	}, WithSupervisor(NewRestartingStrategy()))

	parentPID := rootContext.Spawn(parentProps)
	pair := <-pidCh
	failPID := pair.failPID

	// wait for initial starts
	<-failStart
	<-siblingStart

	// trigger multiple failures and wait for restarts
	for i := 0; i < 3; i++ {
		rootContext.Send(failPID, "fail")
		select {
		case <-failStart:
		case <-time.After(time.Second):
			t.Fatalf("failing child did not restart in time on iteration %d", i)
		}
	}

	// give time for any unexpected sibling restart
	select {
	case <-siblingStart:
		t.Fatalf("sibling child was unexpectedly restarted")
	case <-time.After(100 * time.Millisecond):
	}

	if got, want := atomic.LoadInt32(&failing.starts), int32(4); got != want {
		t.Fatalf("expected failing child to start %d times, got %d", want, got)
	}

	if got := atomic.LoadInt32(&sibling.starts); got != 1 {
		t.Fatalf("expected sibling to start once, got %d", got)
	}

	rootContext.Stop(parentPID)
}
