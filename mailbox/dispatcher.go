package mailbox

type Dispatcher interface {
	Schedule(fn func())
	Throughput() int
}

type goroutineDispatcher int

func (goroutineDispatcher) Schedule(fn func()) {
	go fn()
}

func (d goroutineDispatcher) Throughput() int {
	return int(d)
}

func NewDefaultDispatcher(throughput int) Dispatcher {
	return goroutineDispatcher(throughput)
}
