package actor

// Dispatcher schedules work for actors and reports processing throughput.
type Dispatcher interface {
	// Schedule enqueues the provided function for execution.
	Schedule(fn func())
	// Throughput returns the number of messages processed per scheduling pass.
	Throughput() int
}

type goroutineDispatcher int

var _ Dispatcher = goroutineDispatcher(0)

// Schedule executes the function asynchronously on a new goroutine.
func (goroutineDispatcher) Schedule(fn func()) {
	go fn()
}

// Throughput returns the number of messages processed per pass for the goroutine dispatcher.
func (d goroutineDispatcher) Throughput() int {
	return int(d)
}

// NewDefaultDispatcher creates a Dispatcher that schedules work on separate goroutines.
func NewDefaultDispatcher(throughput int) Dispatcher {
	return goroutineDispatcher(throughput)
}

type synchronizedDispatcher int

var _ Dispatcher = synchronizedDispatcher(0)

// Schedule runs the function synchronously on the calling goroutine.
func (synchronizedDispatcher) Schedule(fn func()) {
	fn()
}

// Throughput returns the number of messages processed per pass for the synchronized dispatcher.
func (d synchronizedDispatcher) Throughput() int {
	return int(d)
}

// NewSynchronizedDispatcher creates a Dispatcher that executes work sequentially.
func NewSynchronizedDispatcher(throughput int) Dispatcher {
	return synchronizedDispatcher(throughput)
}
