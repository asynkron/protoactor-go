// Package eventstream implements a simple publish-subscribe event stream.
package eventstream

import (
	"sync"
	"sync/atomic"
)

// Handler defines a callback function that must be passed when subscribing.
type Handler func(interface{})

// Predicate is a function used to filter messages before being forwarded to a subscriber.
type Predicate func(evt interface{}) bool

// EventStream is a threadsafe publish-subscribe message bus.
type EventStream struct {
	sync.RWMutex

	// slice containing our subscriptions
	subscriptions []*Subscription

	// Atomically maintained elements counter
	counter int32
}

// NewEventStream creates and returns an EventStream.
func NewEventStream() *EventStream {
	es := &EventStream{
		subscriptions: []*Subscription{},
	}

	return es
}

// Subscribe attaches the given handler to the EventStream.
func (es *EventStream) Subscribe(handler Handler) *Subscription {
	sub := &Subscription{
		handler: handler,
		active:  1,
	}

	es.Lock()
	defer es.Unlock()

	sub.id = es.counter
	es.counter++
	es.subscriptions = append(es.subscriptions, sub)

	return sub
}

// SubscribeWithPredicate creates a Subscription and sets a predicate to filter messages.
// It returns a pointer to the Subscription.
func (es *EventStream) SubscribeWithPredicate(handler Handler, p Predicate) *Subscription {
	sub := es.Subscribe(handler)
	sub.p = p

	return sub
}

// Unsubscribe removes the subscription from the EventStream.
func (es *EventStream) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}

	if sub.IsActive() {
		es.Lock()
		defer es.Unlock()

		if sub.Deactivate() {
			if es.counter == 0 {
				es.subscriptions = nil

				return
			}

			l := es.counter - 1
			es.subscriptions[sub.id] = es.subscriptions[l]
			es.subscriptions[sub.id].id = sub.id
			es.subscriptions[l] = nil
			es.subscriptions = es.subscriptions[:l]
			es.counter--

			if es.counter == 0 {
				es.subscriptions = nil
			}
		}
	}
}

// Publish sends the event to all active subscribers.
func (es *EventStream) Publish(evt interface{}) {
	subs := make([]*Subscription, 0, es.Length())
	es.RLock()
	for _, sub := range es.subscriptions {
		if sub.IsActive() {
			subs = append(subs, sub)
		}
	}
	es.RUnlock()

	for _, sub := range subs {
		// there is a subscription predicate and it didn't pass, return
		if sub.p != nil && !sub.p(evt) {
			continue
		}

		// finally here, lets execute our handler
		sub.handler(evt)
	}
}

// Length returns the number of subscribers currently in the stream.
func (es *EventStream) Length() int32 {
	es.RLock()
	defer es.RUnlock()
	return es.counter
}

// Subscription represents a registered handler and its state.
//
// It can be passed to Unsubscribe when the observer is no longer interested in receiving messages.
type Subscription struct {
	id      int32
	handler Handler
	p       Predicate
	active  uint32
}

// Activate sets the Subscription as active. It returns true if the state changed.
func (s *Subscription) Activate() bool {
	return atomic.CompareAndSwapUint32(&s.active, 0, 1)
}

// Deactivate sets the Subscription as inactive. It returns true if the state changed.
func (s *Subscription) Deactivate() bool {
	return atomic.CompareAndSwapUint32(&s.active, 1, 0)
}

// IsActive reports whether the Subscription is active.
func (s *Subscription) IsActive() bool {
	return atomic.LoadUint32(&s.active) == 1
}
