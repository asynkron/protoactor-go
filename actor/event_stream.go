package actor

import (
	"log"
	"sync"
)

type eventStream struct {
	sync.RWMutex
	subscriptions []*Subscription
}

var (
	EventStream = &eventStream{}
)

type Action func(msg interface{})
type Predicate func(msg interface{}) bool
type Subscription struct {
	action Action
}

func init() {
	EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetter); ok {
			log.Printf("[DeadLetter] %v got %+v from %v", deadLetter.PID, deadLetter.Message, deadLetter.Sender)
		}
	})
}

func (es *eventStream) Subscribe(action Action) *Subscription {
	es.Lock()
	sub := &Subscription{
		action: action,
	}
	es.subscriptions = append(es.subscriptions, sub)
	es.Unlock()
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
