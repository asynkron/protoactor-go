package actor

type Dispatcher interface {
	Schedule(runner MailboxRunner)
	Throughput() int
}

type goroutineDispatcher int

func (goroutineDispatcher) Schedule(runner MailboxRunner) {
	go runner()
}

func (d goroutineDispatcher) Throughput() int {
	return int(d)
}

var (
	defaultDispatcher Dispatcher = goroutineDispatcher(300)
)

func NewDefaultDispatcher(throughput int) Dispatcher {
	return goroutineDispatcher(throughput)
}
