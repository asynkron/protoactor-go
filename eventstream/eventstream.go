package eventstream

import (
	"sync"
)

// Predicate is a function used to filter messages before being forwarded to a subscriber
type Predicate func(evt interface{}) bool

var es = &EventStream{}

func Subscribe(fn func(evt interface{})) *Subscription {
	return es.Subscribe(fn)
}

func Unsubscribe(sub *Subscription) {
	es.Unsubscribe(sub)
}

func Publish(event interface{}) {
	es.Publish(event)
}

func PublishUnsafe(evt interface{}) {
	es.PublishUnsafe(evt)
}

type EventStream struct {
	sync.RWMutex
	subscriptions []*Subscription
}

func (es *EventStream) Subscribe(fn func(evt interface{})) *Subscription {
	es.Lock()
	defer es.Unlock()

	sub := &Subscription{
		es: es,
		i:  len(es.subscriptions),
		fn: fn,
	}
	es.subscriptions = append(es.subscriptions, sub)
	return sub
}

func (es *EventStream) Unsubscribe(sub *Subscription) {
	if sub.i == -1 {
		return
	}

	es.Lock()
	defer es.Unlock()
	i := sub.i
	l := len(es.subscriptions) - 1

	if l == -1 {
		es.subscriptions = nil
		sub.i = -1
		return
	}

	es.subscriptions[i] = es.subscriptions[l]
	es.subscriptions[i].i = i
	es.subscriptions[l] = nil
	es.subscriptions = es.subscriptions[:l]
	sub.i = -1

	// TODO(SGC): implement resizing
	if len(es.subscriptions) == 0 {
		es.subscriptions = nil
	}
}

func (es *EventStream) Publish(evt interface{}) {
	es.RLock()
	defer es.RUnlock()

	es.PublishUnsafe(evt)
}

func (es *EventStream) PublishUnsafe(evt interface{}) {
	for _, s := range es.subscriptions {
		if s.p == nil || s.p(evt) {
			s.fn(evt)
		}
	}
}

// Subscription is returned from the Subscribe function.
//
// This value and can be passed to Unsubscribe when the observer is no longer interested in receiving messages
type Subscription struct {
	es *EventStream
	i  int
	fn func(event interface{})
	p  Predicate
}

// WithPredicate sets a predicate to filter messages passed to the subscriber
func (s *Subscription) WithPredicate(p Predicate) *Subscription {
	s.es.Lock()
	defer s.es.Unlock()

	s.p = p
	return s
}
