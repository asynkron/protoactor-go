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

func (ps *eventStream) Unsubscribe(sub *Subscription) {
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

func (ps *eventStream) Publish(evt Event) {
	ps.RLock()
	defer ps.RUnlock()

	for _, s := range ps.subscriptions {
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
