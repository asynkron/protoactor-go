package actor

import (
	"log/slog"

	"github.com/asynkron/protoactor-go/eventstream"
)

// SupervisorEvent is sent on the EventStream when a supervisor have applied a directive to a failing child actor
type SupervisorEvent struct {
	Child     *PID
	Reason    interface{}
	Directive Directive
}

// SubscribeSupervision subscribes to supervision events on the given actor system's
// event stream. It returns the subscription so callers can unsubscribe later.
func SubscribeSupervision(actorSystem *ActorSystem) *eventstream.Subscription {
	return actorSystem.EventStream.Subscribe(func(evt interface{}) {
		if supervisorEvent, ok := evt.(*SupervisorEvent); ok {
			actorSystem.Logger().Debug("[SUPERVISION]", slog.Any("actor", supervisorEvent.Child), slog.Any("directive", supervisorEvent.Directive), slog.Any("reason", supervisorEvent.Reason))
		}
	})
}
