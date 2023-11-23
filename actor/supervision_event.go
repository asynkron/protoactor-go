package actor

import (
	"log/slog"
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
			actorSystem.Logger().Debug("[SUPERVISION]", slog.Any("actor", supervisorEvent.Child), slog.Any("directive", supervisorEvent.Directive), slog.Any("reason", supervisorEvent.Reason))
		}
	})
}
