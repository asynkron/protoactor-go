package gam

type SystemMessage interface {
	systemMessage()
}

type Stop struct{}

func (*Stop) systemMessage() {}

type Watch struct {
	Watcher ActorRef
}

func (*Watch) systemMessage() {}

type Unwatch struct {
	Watcher ActorRef
}

func (*Unwatch) systemMessage() {}

type OtherStopped struct {
	Who ActorRef
}

func (*OtherStopped) systemMessage() {}

type Failure struct {
	Who    ActorRef
	Reason interface{}
}

func (*Failure) systemMessage() {}

type Restart struct{}

func (*Restart) systemMessage() {}

type Resume struct{}

func (*Resume) systemMessage() {}
