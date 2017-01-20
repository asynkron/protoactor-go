package actor

import "log"

type SupervisorEvent struct {
	Child     *PID
	Reason    interface{}
	Directive Directive
}

var (
	supervisionSubscriber *Subscription
)

func init() {
	supervisionSubscriber = EventStream.Subscribe(func(msg interface{}) {
		if supervisorEvent, ok := msg.(*SupervisorEvent); ok {
			log.Printf("[ACTOR] [SUPERVISION] - Actor: '%v' Directive: '%v' Reason: '%v' ", supervisorEvent.Child, supervisorEvent.Directive.String(), supervisorEvent.Reason)
		}
	})
}
