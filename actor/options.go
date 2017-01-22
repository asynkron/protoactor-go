package actor

import "github.com/AsynkronIT/protoactor-go/eventstream"

type optionFn func()

// WithDeadLetterSubscriber option replaces the default DeadLetterEvent event subscriber with fn.
//
// fn will only receive *DeadLetterEvent messages
//
// Specifying nil will clear the existing.
func WithDeadLetterSubscriber(fn func(evt interface{})) optionFn {
	return func() {
		if deadLetterSubscriber != nil {
			eventstream.Unsubscribe(deadLetterSubscriber)
		}
		if fn != nil {
			deadLetterSubscriber = eventstream.Subscribe(fn).
				WithPredicate(func(m interface{}) bool {
					_, ok := m.(*DeadLetterEvent)
					return ok
				})
		}
	}
}

// WithSupervisorSubscriber option replaces the default SupervisorEvent event subscriber with fn.
//
// fn will only receive *SupervisorEvent messages
//
// Specifying nil will clear the existing.
func WithSupervisorSubscriber(fn func(evt interface{})) optionFn {
	return func() {
		if supervisionSubscriber != nil {
			eventstream.Unsubscribe(supervisionSubscriber)
		}
		if fn != nil {
			supervisionSubscriber = eventstream.Subscribe(fn).
				WithPredicate(func(m interface{}) bool {
					_, ok := m.(*SupervisorEvent)
					return ok
				})
		}
	}
}

// SetOptions is used to configure the actor system
func SetOptions(opts ...optionFn) {
	for _, opt := range opts {
		opt()
	}
}
