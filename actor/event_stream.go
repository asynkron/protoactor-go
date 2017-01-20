package actor

import (
	"sync"
)

type eventStream struct {
	sync.RWMutex
	subscriptions []*Subscription
}

var (
	EventStream = &eventStream{}
)

// SubscriberFunc is the signature of an EventStream subscriber function
type SubscriberFunc func(msg interface{})

type Predicate func(msg interface{}) bool

// Subscription is returned from the Subscribe function.
//
// This value and can be passed to Unsubscribe when the observer is no longer interested in receiving messages
type Subscription struct {
	i  int
	fn SubscriberFunc
	p  Predicate
}

// WithPredicate sets a predicate to filter messages passed to the subscriber
func (s *Subscription) WithPredicate(p Predicate) *Subscription {
	s.p = p
	return s
}

func (es *eventStream) Subscribe(fn SubscriberFunc) *Subscription {
	es.Lock()
	sub := &Subscription{
		i:  len(es.subscriptions),
		fn: fn,
	}
	es.subscriptions = append(es.subscriptions, sub)
	es.Unlock()
	return sub
}

func (es *eventStream) SubscribePID(pid *PID) *Subscription {
	return es.Subscribe(pid.Tell)
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
		if s.p == nil || s.p(message) {
			s.fn(message)
		}
	}
}
