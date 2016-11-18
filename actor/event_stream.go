package actor

import "sync"

type Predicate func(msg interface{}) bool
type subscription struct {
	pid       *PID
	predicate Predicate
}

type eventStream struct {
	sync.RWMutex
	subscriptions []*subscription
}

var EventStream = &eventStream{}

func (es *eventStream) Subscribe(subscriber *PID, predicate Predicate) {
	es.Lock()
	defer es.Unlock()
	es.subscriptions = append(es.subscriptions, &subscription{
		pid:       subscriber,
		predicate: predicate,
	})
}

func (es *eventStream) Publish(message interface{}) {
	for _, s := range es.subscriptions {
		if s.predicate(message) {
			s.pid.Tell(message)
		}
	}
}
