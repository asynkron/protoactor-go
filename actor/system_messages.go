package actor

//SystemMessage is a special type of messages passed to control the actor lifecycles
type SystemMessage interface {
	systemMessage()
}

func (*Stop) systemMessage() {}

func (*Watch) systemMessage() {}

func (*Unwatch) systemMessage() {}

func (*Terminated) systemMessage() {}

func (*Failure) systemMessage() {}

func (*Restart) systemMessage() {}

func (*Resume) systemMessage() {}

type Restart struct{}

type Resume struct{}

type Stop struct{}

type Watch struct {
	Watcher *PID
}
type Unwatch struct {
	Watcher *PID
}
type Terminated struct {
	Who *PID
}

type Failure struct {
	Who    *PID
	Reason interface{}
}
