package plugin

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type PassivationAware interface {
	Init(*actor.PID, time.Duration)
	Reset(time.Duration)
	Cancel()
}

type PassivationHolder struct {
	timer *time.Timer
	done  int32
}

func (state *PassivationHolder) Reset(duration time.Duration) {
	if state.timer == nil {
		log.Fatalf("Cannot reset passivation of a non-started actor")
	}
	if atomic.LoadInt32(&state.done) == 0 {
		state.timer.Reset(duration)
	}
}

func (state *PassivationHolder) Init(pid *actor.PID, duration time.Duration) {
	state.timer = time.NewTimer(duration)
	state.done = 0
	go func() {
		select {
		case <-state.timer.C:
			actor.EmptyRootContext.Stop(pid)
			atomic.StoreInt32(&state.done, 1)
			break
		}
	}()
}

func (state *PassivationHolder) Cancel() {
	if state.timer != nil {
		state.timer.Stop()
	}
}

type PassivationPlugin struct {
	Duration time.Duration
}

func (pp *PassivationPlugin) OnStart(ctx actor.ReceiverContext) {
	if a, ok := ctx.Actor().(PassivationAware); ok {
		a.Init(ctx.Self(), pp.Duration)
	}
}

func (pp *PassivationPlugin) OnOtherMessage(ctx actor.ReceiverContext, env *actor.MessageEnvelope) {
	if p, ok := ctx.Actor().(PassivationAware); ok {
		switch env.Message.(type) {
		case *actor.Stopped:
			p.Cancel()
		default:
			p.Reset(pp.Duration)
		}
	}
}
