package gam

type SystemMessage interface {
	systemMessage()
}

type stop struct{}

func (*stop) systemMessage() {}

type watch struct {
	Watcher ActorRef
}

func (*watch) systemMessage() {}

type unwatch struct {
	Watcher ActorRef
}

func (*unwatch) systemMessage() {}

type otherStopped struct {
	Who ActorRef
}

func (*otherStopped) systemMessage() {}

type failure struct {
	Who    ActorRef
	Reason interface{}
}

func (*failure) systemMessage() {}

type restart struct{}

func (*restart) systemMessage() {}

type resume struct{}

func (*resume) systemMessage() {}
