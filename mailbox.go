package actor

type MailboxProducer func(func(interface{}),func(SystemMessage)) Mailbox
type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message SystemMessage)
	Suspend()
	Resume()
	RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage))
}

const (
	MailboxIdle    = iota
	MailboxRunning = iota
)
const (
	MailboxHasNoMessages   = iota
	MailboxHasMoreMessages = iota
)

