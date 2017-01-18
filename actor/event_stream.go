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
	i      int
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
		i:      len(es.subscriptions),
		action: action,
	}
	es.subscriptions = append(es.subscriptions, sub)
	es.Unlock()
	return sub
}

func (es *eventStream) SubscribePID(pid *PID, predicate Predicate) *Subscription {
	return es.Subscribe(func(msg interface{}) {
		if predicate(msg) {
			pid.Tell(msg)
		}
	})
}

func (es *eventStream) Unsubscribe(sub *Subscription) {
	if sub.i == -1 {
		return
	}

	es.Lock()
	i := sub.i
	l := len(es.subscriptions) - 1

	es.subscriptions[i] = es.subscriptions[l]
	es.subscriptions[i].i = i
	es.subscriptions[l] = nil
	es.subscriptions = es.subscriptions[:l]
	sub.i = -1

	// TODO(SGC): implement resizing
	if len(es.subscriptions) == 0 {
		es.subscriptions = nil
	}

	es.Unlock()
}

func (es *eventStream) Publish(message interface{}) {
	es.RLock()
	defer es.RUnlock()

	for _, s := range es.subscriptions {
		s.action(message)
	}
}
