package actor

type Dispatcher interface {
	Schedule(fn func())
	Throughput() int
}

type goroutineDispatcher int

var _ Dispatcher = goroutineDispatcher(0)

func (goroutineDispatcher) Schedule(fn func()) {
	go fn()
}

func (d goroutineDispatcher) Throughput() int {
	return int(d)
}

func NewDefaultDispatcher(throughput int) Dispatcher {
	return goroutineDispatcher(throughput)
}

type synchronizedDispatcher int

var _ Dispatcher = synchronizedDispatcher(0)

func (synchronizedDispatcher) Schedule(fn func()) {
	fn()
}

func (d synchronizedDispatcher) Throughput() int {
	return int(d)
}

func NewSynchronizedDispatcher(throughput int) Dispatcher {
	return synchronizedDispatcher(throughput)
}
