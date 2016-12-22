package actor

type Dispatcher interface {
	Schedule(runner MailboxRunner)
	Throughput() int
}

type goroutineDispatcher struct {
	throughput int
}

func (*goroutineDispatcher) Schedule(runner MailboxRunner) {
	go runner()
}

func (d *goroutineDispatcher) Throughput() int {
	return d.throughput
}

var (
	defaultDispatcher = &goroutineDispatcher{
		throughput: 300,
	}
)

func NewDefaultDispatcher(throughput int) Dispatcher {
	return &goroutineDispatcher{
		throughput: throughput,
	}
}
