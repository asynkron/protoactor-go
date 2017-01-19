package actor

type AutoReceiveMessage interface {
	AutoReceiveMessage()
}

type NotInfluenceReceiveTimeout interface {
	NotInfluenceReceiveTimeout()
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

//TODO: make private?
type Failure struct {
	Who        *PID
	Reason     interface{}
	ChildStats *ChildRestartStats
}

func (*Restarting) AutoReceiveMessage() {}
func (*Stopping) AutoReceiveMessage()   {}
func (*Stopped) AutoReceiveMessage()    {}
func (*PoisonPill) AutoReceiveMessage() {}

func (*Started) SystemMessage()        {}
func (*Stop) SystemMessage()           {}
func (*Watch) SystemMessage()          {}
func (*Unwatch) SystemMessage()        {}
func (*Terminated) SystemMessage()     {}
func (*Failure) SystemMessage()        {}
func (*Restart) SystemMessage()        {}
func (*ResumeMailbox) SystemMessage()  {}
func (*SuspendMailbox) SystemMessage() {}

var (
	restartingMessage     interface{} = &Restarting{}
	stoppingMessage       interface{} = &Stopping{}
	stoppedMessage        interface{} = &Stopped{}
	poisonPillMessage     interface{} = &PoisonPill{}
	receiveTimeoutMessage interface{} = &ReceiveTimeout{}
)

var (
	restartMessage        SystemMessage = &Restart{}
	startedMessage        SystemMessage = &Started{}
	stopMessage           SystemMessage = &Stop{}
	resumeMailboxMessage  SystemMessage = &ResumeMailbox{}
	suspendMailboxMessage SystemMessage = &SuspendMailbox{}
)
