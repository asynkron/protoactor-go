package actor

type optionFn func()

// WithDeadLetterSubscriber option replaces the default DeadLetterEvent EventStream subscriber with fn.
//
// fn will only receive *DeadLetterEvent messages
//
// Specifying nil will clear the existing.
func WithDeadLetterSubscriber(fn SubscriberFunc) optionFn {
	return func() {
		if deadLetterSubscriber != nil {
			EventStream.Unsubscribe(deadLetterSubscriber)
		}
		if fn != nil {
			deadLetterSubscriber = EventStream.Subscribe(fn).
				WithPredicate(func(m interface{}) bool {
					_, ok := m.(*DeadLetterEvent)
					return ok
				})
		}
	}
}

// WithSupervisorSubscriber option replaces the default SupervisorEvent EventStream subscriber with fn.
//
// fn will only receive *SupervisorEvent messages
//
// Specifying nil will clear the existing.
func WithSupervisorSubscriber(fn SubscriberFunc) optionFn {
	return func() {
		if supervisionSubscriber != nil {
			EventStream.Unsubscribe(supervisionSubscriber)
		}
		if fn != nil {
			supervisionSubscriber = EventStream.Subscribe(fn).
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
