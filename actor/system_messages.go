package actor

//SystemMessage is a special type of messages passed to control the actor lifecycles
type SystemMessage interface {
	SystemMessage()
}

func (*Stop) SystemMessage() {}

func (*Watch) SystemMessage() {}

func (*Unwatch) SystemMessage() {}

func (*Terminated) SystemMessage() {}

func (*Failure) SystemMessage() {}

func (*Restart) SystemMessage() {}

func (*ResumeMailbox) SystemMessage() {}

func (*SuspendMailbox) SystemMessage() {}

type Restart struct{}

type ResumeMailbox struct{}

type SuspendMailbox struct{}

type Stop struct{}

type Failure struct {
	Who    *PID
	Reason interface{}
}
