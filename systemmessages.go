package actor

type Stop struct{}

func (*Stop) SystemMessage() {}

type Watch struct {
	Watcher ActorRef
}

func (*Watch) SystemMessage() {}

type Unwatch struct {
	Watcher ActorRef
}

func (*Unwatch) SystemMessage() {}

type OtherStopped struct {
	Who ActorRef
}

func (*OtherStopped) SystemMessage() {}

type Failure struct {
	Who    ActorRef
	Reason interface{}
}

func (*Failure) SystemMessage() {}

type Restart struct{}

func (*Restart) SystemMessage() {}

type Resume struct{}

func (*Resume) SystemMessage() {}
