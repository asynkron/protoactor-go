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

type EventStream struct {
	sync.RWMutex
	subscriptions []*Subscription
}

func (es *EventStream) Subscribe(fn func(evt interface{})) *Subscription {
	es.Lock()
	sub := &Subscription{
		es: es,
		i:  len(es.subscriptions),
		fn: fn,
	}
	es.subscriptions = append(es.subscriptions, sub)
	es.Unlock()
	return sub
}

func (ps *EventStream) Unsubscribe(sub *Subscription) {
	if sub.i == -1 {
		return
	}

	ps.Lock()
	i := sub.i
	l := len(ps.subscriptions) - 1

	ps.subscriptions[i] = ps.subscriptions[l]
	ps.subscriptions[i].i = i
	ps.subscriptions[l] = nil
	ps.subscriptions = ps.subscriptions[:l]
	sub.i = -1

	// TODO(SGC): implement resizing
	if len(ps.subscriptions) == 0 {
		ps.subscriptions = nil
	}

	ps.Unlock()
}

func (ps *EventStream) Publish(evt interface{}) {
	ps.RLock()
	defer ps.RUnlock()

	for _, s := range ps.subscriptions {
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
	s.p = p
	s.es.Unlock()
	return s
}
