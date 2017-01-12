package actor

type AutoReceiveMessage interface {
	AutoReceiveMessage()
}

//SystemMessage is a special type of messages passed to control the actor lifecycles
type SystemMessage interface {
	SystemMessage()
}

type ReceiveTimeout struct{}
type Restarting struct{}
type Stopping struct{}
type Stopped struct{}
type Started struct{}

//TODO: add cause and action?
type Restart struct{}

//TODO: add cause and action?
type Stop struct{}
type ResumeMailbox struct{}
type SuspendMailbox struct{}
type Failure struct {
	Who    *PID
	Reason interface{}
}

func (*Restarting) AutoReceiveMessage() {}
func (*Stopping) AutoReceiveMessage()   {}
func (*Stopped) AutoReceiveMessage()    {}
func (*PoisonPill) AutoReceiveMessage() {}
func (*Started) AutoReceiveMessage()    {}

func (*Stop) SystemMessage()           {}
func (*Watch) SystemMessage()          {}
func (*Unwatch) SystemMessage()        {}
func (*Terminated) SystemMessage()     {}
func (*Failure) SystemMessage()        {}
func (*Restart) SystemMessage()        {}
func (*ResumeMailbox) SystemMessage()  {}
func (*SuspendMailbox) SystemMessage() {}

var (
	restartMessage        SystemMessage = &Restart{}
	stopMessage           SystemMessage = &Stop{}
	resumeMailboxMessage  SystemMessage = &ResumeMailbox{}
	suspendMailboxMessage SystemMessage = &SuspendMailbox{}
)
