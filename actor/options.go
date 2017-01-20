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

// SetOptions
func SetOptions(opts ...optionFn) {
	for _, opt := range opts {
		opt()
	}
}
