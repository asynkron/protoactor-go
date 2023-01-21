package eventstream

import (
	"sync"
	"sync/atomic"
)

// Handler defines a callback function that must be pass when subscribing.
type Handler func(interface{})

// Predicate is a function used to filter messages before being forwarded to a subscriber
type Predicate func(evt interface{}) bool

type EventStream struct {
	sync.RWMutex

	// slice containing our subscriptions
	subscriptions []*Subscription

	// Atomically maintained elements counter
	counter int32
}

// Create a new EventStream value and returns it back.
func NewEventStream() *EventStream {
	es := &EventStream{
		subscriptions: []*Subscription{},
	}

	return es
}

// Subscribe the given handler to the EventStream
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

// SubscribeWithPredicate creates a new Subscription value and sets a predicate to filter messages passed to
// the subscriber, it returns a pointer to the Subscription value
func (es *EventStream) SubscribeWithPredicate(handler Handler, p Predicate) *Subscription {
	sub := es.Subscribe(handler)
	sub.p = p

	return sub
}

// Unsubscribes the given subscription from the EventStream
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

// Publishes the given event to all the subscribers in the stream
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

// Returns an integer that represents the current number of subscribers to the stream
func (es *EventStream) Length() int32 {
	return atomic.LoadInt32(&es.counter)
}

// Subscription is returned from the Subscribe function.
//
// This value and can be passed to Unsubscribe when the observer is no longer interested in receiving messages
type Subscription struct {
	id      int32
	handler Handler
	p       Predicate
	active  uint32
}

// Activates the Subscription setting its active flag as 1, if the subscription
// was already active it returns false, true otherwise
func (s *Subscription) Activate() bool {
	return atomic.CompareAndSwapUint32(&s.active, 0, 1)
}

// Deactivates the Subscription setting its active flag as 0, if the subscription
// was already inactive it returns false, true otherwise
func (s *Subscription) Deactivate() bool {
	return atomic.CompareAndSwapUint32(&s.active, 1, 0)
}

// Returns true if the active flag of the Subscription is set as 1
// otherwise it returns false
func (s *Subscription) IsActive() bool {
	return atomic.LoadUint32(&s.active) == 1
}
