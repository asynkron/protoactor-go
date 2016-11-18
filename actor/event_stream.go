package actor

import (
	"log"
	"sync"
)

type Action func(msg interface{})
type Predicate func(msg interface{}) bool
type subscription struct {
	action Action
}

type eventStream struct {
	sync.RWMutex
	subscriptions []*subscription
}

var EventStream = &eventStream{}

func init() {
	EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetter); ok {
			log.Printf("[DeadLetter] %v got %+v", deadLetter.PID, deadLetter.Message)
		}
	})
}

func (es *eventStream) Subscribe(action Action) {
	es.Lock()
	defer es.Unlock()
	es.subscriptions = append(es.subscriptions, &subscription{
		action: action,
	})
}

func (es *eventStream) SubscribePID(predicate Predicate, pid *PID) {
	es.Subscribe(func(msg interface{}) {
		if predicate(msg) {
			pid.Tell(msg)
		}
	})
}

func (es *eventStream) Publish(message interface{}) {
	for _, s := range es.subscriptions {
		s.action(message)
	}
}
