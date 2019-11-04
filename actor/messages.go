package actor

import "github.com/otherview/protoactor-go/mailbox"

// An AutoReceiveMessage is a special kind of user message that will be handled in some way automatically by the actor
type AutoReceiveMessage interface {
	AutoReceiveMessage()
}

// NotInfluenceReceiveTimeout messages will not reset the ReceiveTimeout timer of an actor that receives the message
type NotInfluenceReceiveTimeout interface {
	NotInfluenceReceiveTimeout()
}

// A SystemMessage message is reserved for specific lifecycle messages used by the actor system
type SystemMessage interface {
	SystemMessage()
}

// A ReceiveTimeout message is sent to an actor after the Context.ReceiveTimeout duration has expired
type ReceiveTimeout struct{}

// A Restarting message is sent to an actor when the actor is being restarted by the system due to a failure
type Restarting struct{}

// A Stopping message is sent to an actor prior to the actor being stopped
type Stopping struct{}

// A Stopped message is sent to the actor once it has been stopped. A stopped actor will receive no further messages
type Stopped struct{}

// A Started message is sent to an actor once it has been started and ready to begin receiving messages.
type Started struct{}

// Restart is message sent by the actor system to control the lifecycle of an actor
type Restart struct{}

type Failure struct {
	Who          *PID
	Reason       interface{}
	RestartStats *RestartStatistics
	Message      interface{}
}

type continuation struct {
	message interface{}
	f       func()
}

func (*Restarting) AutoReceiveMessage() {}
func (*Stopping) AutoReceiveMessage()   {}
func (*Stopped) AutoReceiveMessage()    {}
func (*PoisonPill) AutoReceiveMessage() {}

func (*Started) SystemMessage()      {}
func (*Stop) SystemMessage()         {}
func (*Watch) SystemMessage()        {}
func (*Unwatch) SystemMessage()      {}
func (*Terminated) SystemMessage()   {}
func (*Failure) SystemMessage()      {}
func (*Restart) SystemMessage()      {}
func (*continuation) SystemMessage() {}

var (
	restartingMessage     interface{} = &Restarting{}
	stoppingMessage       interface{} = &Stopping{}
	stoppedMessage        interface{} = &Stopped{}
	poisonPillMessage     interface{} = &PoisonPill{}
	receiveTimeoutMessage interface{} = &ReceiveTimeout{}
)

var (
	restartMessage        interface{} = &Restart{}
	startedMessage        interface{} = &Started{}
	stopMessage           interface{} = &Stop{}
	resumeMailboxMessage  interface{} = &mailbox.ResumeMailbox{}
	suspendMailboxMessage interface{} = &mailbox.SuspendMailbox{}
)
