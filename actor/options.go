package actor

type optionFn func()

// SetOptions is used to configure the actor system
func SetOptions(opts ...optionFn) {
	for _, opt := range opts {
		opt()
	}
}
