package actor

type ReceiveUserMessage func(interface{})
type ReceiveSystemMessage func(SystemMessage)

type MailboxRunner func()
type MailboxProducer func() Mailbox
type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message SystemMessage)
	RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher)
}

const (
	mailboxIdle    int32 = iota
	mailboxRunning int32 = iota
)
const (
	mailboxHasNoMessages   int32 = iota
	mailboxHasMoreMessages int32 = iota
)
