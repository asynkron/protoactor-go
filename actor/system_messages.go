package actor

type SystemMessage interface {
	systemMessage()
}

type stop struct{}

func (*stop) systemMessage() {}

type watch struct {
	Watcher *PID
}

func (*watch) systemMessage() {}

type unwatch struct {
	Watcher *PID
}

func (*unwatch) systemMessage() {}

type otherStopped struct {
	Who *PID
}

func (*otherStopped) systemMessage() {}

type failure struct {
	Who    *PID
	Reason interface{}
}

func (*failure) systemMessage() {}

type restart struct{}

func (*restart) systemMessage() {}

type resume struct{}

func (*resume) systemMessage() {}
