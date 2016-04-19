package interfaces

type MailboxProducer func(func(interface{}),func(SystemMessage)) Mailbox
type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message SystemMessage)
	Suspend()
	Resume()
}