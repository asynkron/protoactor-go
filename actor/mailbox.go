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
	mailboxIdle int32 = iota
	mailboxRunning int32 = iota
)
const (
	mailboxHasNoMessages int32 = iota
	mailboxHasMoreMessages int32 = iota
)
