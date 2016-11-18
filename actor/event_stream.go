package actor

import (
	"log"
	"sync"
)

type Action func(msg interface{})
type Predicate func(msg interface{}) bool
type Subscription struct {
	action Action
}

type eventStream struct {
	sync.RWMutex
	subscriptions []*Subscription
}

var EventStream = &eventStream{}

func init() {
	EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetter); ok {
			log.Printf("[DeadLetter] %v got %+v", deadLetter.PID, deadLetter.Message)
		}
	})
}

func (es *eventStream) Subscribe(action Action) *Subscription {
	es.Lock()
	defer es.Unlock()
	sub := &Subscription{
		action: action,
	}
	es.subscriptions = append(es.subscriptions, sub)
	return sub
}

func (es *eventStream) SubscribePID(predicate Predicate, pid *PID) *Subscription {
	return es.Subscribe(func(msg interface{}) {
		if predicate(msg) {
			pid.Tell(msg)
		}
	})
}

func (es *eventStream) Unsubscribe(sub *Subscription) {

}

func (es *eventStream) Publish(message interface{}) {
	for _, s := range es.subscriptions {
		s.action(message)
	}
}
