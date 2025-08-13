package actor

// ResumeMailbox is message sent by the actor system to resume mailbox processing.
//
// This will not be forwarded to the Receive method
type ResumeMailbox struct{}

// SuspendMailbox is message sent by the actor system to suspend mailbox processing.
//
// This will not be forwarded to the Receive method
type SuspendMailbox struct{}

// MailboxMessage marks a message that can be processed by a mailbox.
type MailboxMessage interface {
	MailboxMessage()
}

// MailboxMessage implements the MailboxMessage interface.
func (*SuspendMailbox) MailboxMessage() {}

// MailboxMessage implements the MailboxMessage interface.
func (*ResumeMailbox) MailboxMessage() {}

// InfrastructureMessage is a marker for all built in Proto.Actor messages
type InfrastructureMessage interface {
	InfrastructureMessage()
}

// IgnoreDeadLetterLogging messages are not logged in deadletter log
type IgnoreDeadLetterLogging interface {
	IgnoreDeadLetterLogging()
}

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

// Failure message is sent to an actor parent when an exception is thrown by one of its methods
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

// GetAutoResponse returns the auto-response for a Touch message.
func (*Touch) GetAutoResponse(ctx Context) interface{} {
	return &Touched{
		Who: ctx.Self(),
	}
}

// AutoReceiveMessage marks Restarting as an automatically handled message.
func (*Restarting) AutoReceiveMessage() {}

// AutoReceiveMessage marks Stopping as an automatically handled message.
func (*Stopping) AutoReceiveMessage() {}

// AutoReceiveMessage marks Stopped as an automatically handled message.
func (*Stopped) AutoReceiveMessage() {}

// AutoReceiveMessage marks PoisonPill as an automatically handled message.
func (*PoisonPill) AutoReceiveMessage() {}

// SystemMessage marks Started as a system message.
func (*Started) SystemMessage() {}

// SystemMessage marks Stop as a system message.
func (*Stop) SystemMessage() {}

// SystemMessage marks Watch as a system message.
func (*Watch) SystemMessage() {}

// SystemMessage marks Unwatch as a system message.
func (*Unwatch) SystemMessage() {}

// SystemMessage marks Terminated as a system message.
func (*Terminated) SystemMessage() {}

// SystemMessage marks Failure as a system message.
func (*Failure) SystemMessage() {}

// SystemMessage marks Restart as a system message.
func (*Restart) SystemMessage() {}

// SystemMessage marks continuation as a system message.
func (*continuation) SystemMessage() {}

var (
	restartingMessage     AutoReceiveMessage = &Restarting{}
	stoppingMessage       AutoReceiveMessage = &Stopping{}
	stoppedMessage        AutoReceiveMessage = &Stopped{}
	poisonPillMessage     AutoReceiveMessage = &PoisonPill{}
	receiveTimeoutMessage interface{}        = &ReceiveTimeout{}
	restartMessage        SystemMessage      = &Restart{}
	startedMessage        SystemMessage      = &Started{}
	stopMessage           SystemMessage      = &Stop{}
	resumeMailboxMessage  MailboxMessage     = &ResumeMailbox{}
	suspendMailboxMessage MailboxMessage     = &SuspendMailbox{}
	_                     AutoRespond        = &Touch{}
)
