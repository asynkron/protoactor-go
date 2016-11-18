package cluster

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
)

type PassivationAware interface {
	Init(*actor.PID, time.Duration)
	Reset(time.Duration)
	Cancel()
}

func (state *PassivationPlugin) Reset(duration time.Duration) {
	if state.timer == nil {
		log.Fatalf("Cannot reset passivation of a non-started actor")
	}
	if !state.done {
		state.timer.Reset(duration)
	}
}

func (state *PassivationPlugin) Init(pid *actor.PID, duration time.Duration) {
	state.timer = time.NewTimer(duration)
	state.done = false
	go func() {
		select {
		case <-state.timer.C:
			log.Printf("[CLUSTER] Passivating %v", pid.Id)
			pid.Stop()
			state.done = true
			break
		}
	}()
}

func (state *PassivationPlugin) Cancel() {
	if state.timer != nil {
		state.timer.Stop()
	}
}

type PassivationPlugin struct {
	Duration time.Duration
	timer    *time.Timer
	done     bool
}

func (pp *PassivationPlugin) OnStart(ctx actor.Context) {
	pp.Init(ctx.Self(), pp.Duration)
}

func (pp *PassivationPlugin) OnOtherMessage(ctx actor.Context, msg interface{}) {
	switch msg.(type) {
	case *actor.Stopped:
		pp.Cancel()
	default:
		pp.Reset(pp.Duration)
	}
}
