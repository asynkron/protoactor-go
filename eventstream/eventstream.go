package eventstream

import (
	"sync"

	"github.com/google/uuid"
	cmap "github.com/orcaman/concurrent-map"
)

// Predicate is a function used to filter messages before being forwarded to a subscriber
type Predicate func(evt interface{}) bool

var es = &EventStream{
	subscriptions: cmap.New(),
}

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
	subscriptions cmap.ConcurrentMap
}

func (es *EventStream) Subscribe(fn func(evt interface{})) *Subscription {
	uuid := uuid.New().String()
	sub := &Subscription{
		es: es,
		id: uuid,
		fn: fn,
	}
	es.subscriptions.SetIfAbsent(uuid, sub)
	return sub
}

func (es *EventStream) Unsubscribe(sub *Subscription) {
	es.subscriptions.Remove(sub.id)
}

func (es *EventStream) Publish(evt interface{}) {
	for _, key := range es.subscriptions.Keys() {
		i, ok := es.subscriptions.Get(key)
		if ok {
			s, ok := i.(*Subscription)
			if ok && (s.p == nil || s.p(evt)) {
				s.fn(evt)
			}
		}
	}
}

// Subscription is returned from the Subscribe function.
//
// This value and can be passed to Unsubscribe when the observer is no longer interested in receiving messages
type Subscription struct {
	es *EventStream
	id string
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
