package actor

import "github.com/AsynkronIT/protoactor-go/eventstream"

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
			logdbg.Printf("[SUPERVISION] Actor: '%v' Directive: '%v' Reason: '%v' ", supervisorEvent.Child, supervisorEvent.Directive.String(), supervisorEvent.Reason)
		}
	})
}
