package actor

type Dispatcher interface {
	Dispatch(runner MailboxRunner)
}

type goroutineDispatcher struct {
}

func (*goroutineDispatcher) Dispatch(runner MailboxRunner) {
	go runner()
}

var (
	defaultDispatcher = &goroutineDispatcher{}
)
