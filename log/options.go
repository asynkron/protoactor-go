package log

type optionFn func()

// WithEventSubscriber option replaces the default Event subscriber with fn.
//
// Specifying nil will disable logging of events.
func WithEventSubscriber(fn func(evt Event)) optionFn {
	return func() {
		if sub != nil {
			Unsubscribe(sub)
		}
		if fn != nil {
			sub = Subscribe(fn)
		}
	}
}

// SetOptions is used to configure the log system
func SetOptions(opts ...optionFn) {
	for _, opt := range opts {
		opt()
	}
}
