package actor

import "github.com/ivpusic/grpool"

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

type poolDispatcher struct {
	pool       *grpool.Pool
	throughput int
}

func NewPoolDispatcher(workers int, queueSize int, throughput int) Dispatcher {
	pool := grpool.NewPool(workers, queueSize)
	d := &poolDispatcher{
		pool:       pool,
		throughput: throughput,
	}
	return d
}

func (d *poolDispatcher) Schedule(runner MailboxRunner) {
	d.pool.JobQueue <- func() {
		runner()
	}
}

func (d *poolDispatcher) Throughput() int {
	return d.throughput
}
