package actor

import "github.com/ivpusic/grpool"

type Dispatcher interface {
	Schedule(runner MailboxRunner)
}

type goroutineDispatcher struct {
}

func (*goroutineDispatcher) Schedule(runner MailboxRunner) {
	go runner()
}

var (
	defaultDispatcher = &goroutineDispatcher{}
)

type poolDispatcher struct {
	pool *grpool.Pool
}

func NewPoolDispatcher(workers int, queueSize int) Dispatcher {
	pool := grpool.NewPool(workers, queueSize)
	d := &poolDispatcher{
		pool: pool,
	}
	return d
}

func (d *poolDispatcher) Schedule(runner MailboxRunner) {
	d.pool.JobQueue <- func() {
		runner()
	}
}
