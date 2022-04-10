package actor

import (
	"github.com/asynkron/protoactor-go/log"
)

// SupervisorEvent is sent on the EventStream when a supervisor have applied a directive to a failing child actor
type SupervisorEvent struct {
	Child     *PID
	Reason    interface{}
	Directive Directive
}

func SubscribeSupervision(actorSystem *ActorSystem) {
	_ = actorSystem.EventStream.Subscribe(func(evt interface{}) {
		if supervisorEvent, ok := evt.(*SupervisorEvent); ok {
			plog.Debug("[SUPERVISION]", log.Stringer("actor", supervisorEvent.Child), log.Stringer("directive", supervisorEvent.Directive), log.Object("reason", supervisorEvent.Reason))
		}
	})
}
