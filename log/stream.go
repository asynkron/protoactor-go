package log

import "sync"

var es = &eventStream{}

func Subscribe(fn func(evt Event)) *Subscription {
	return es.Subscribe(fn)
}

func Unsubscribe(sub *Subscription) {
	es.Unsubscribe(sub)
}

type eventStream struct {
	sync.RWMutex
	subscriptions []*Subscription
}

func (es *eventStream) Subscribe(fn func(evt Event)) *Subscription {
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

func (es *eventStream) Publish(evt Event) {
	es.RLock()
	defer es.RUnlock()

	for _, s := range es.subscriptions {
		if evt.Level >= s.l {
			s.fn(evt)
		}
	}
}

// Subscription is returned from the Subscribe function.
//
// This value and can be passed to Unsubscribe when the observer is no longer interested in receiving messages
type Subscription struct {
	es *eventStream
	i  int
	fn func(event Event)
	l  Level
}

// WithMinLevel filter messages below the provided level
//
// For example, setting ErrorLevel will only pass error messages. Setting MinLevel will
// allow all messages, and is the default.
func (s *Subscription) WithMinLevel(level Level) *Subscription {
	s.es.Lock()
	s.l = level
	s.es.Unlock()
	return s
}
