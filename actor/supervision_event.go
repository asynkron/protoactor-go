package actor

import (
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/log"
)

type SupervisorEvent struct {
	Child     *PID
	Reason    interface{}
	Directive Directive
}

var (
	supervisionSubscriber *eventstream.Subscription
)

func init() {
	supervisionSubscriber = eventstream.Subscribe(func(evt interface{}) {
		if supervisorEvent, ok := evt.(*SupervisorEvent); ok {
			plog.Debug("[SUPERVISION]", log.Stringer("actor", supervisorEvent.Child), log.Stringer("directive", supervisorEvent.Directive), log.Object("reason", supervisorEvent.Reason))
		}
	})
}
