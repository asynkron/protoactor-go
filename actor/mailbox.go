package actor

type MailboxProducer func() Mailbox
type Mailbox interface {
	PostUserMessage(message UserMessage)
	PostSystemMessage(message SystemMessage)
	Suspend()
	Resume()
	RegisterHandlers(userInvoke func(UserMessage), systemInvoke func(SystemMessage))
}

const (
	mailboxIdle    int32 = iota
	mailboxRunning int32 = iota
)
const (
	mailboxHasNoMessages   int32 = iota
	mailboxHasMoreMessages int32 = iota
)
