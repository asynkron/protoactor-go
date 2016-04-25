package actor

type MailboxProducer func() Mailbox
type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message SystemMessage)
	Suspend()
	Resume()
	RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage))
}

const (
	MailboxIdle    int32 = iota
	MailboxRunning int32 = iota
)
const (
	MailboxHasNoMessages   int32 = iota
	MailboxHasMoreMessages int32 = iota
)
