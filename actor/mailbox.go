package actor

const (
	MailboxIdle    = iota
	MailboxRunning = iota
)
const (
	MailboxHasNoMessages   = iota
	MailboxHasMoreMessages = iota
)

type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message interface{})
}