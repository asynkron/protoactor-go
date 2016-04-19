package actor

const (
	MailboxIdle    = iota
	MailboxRunning = iota
)
const (
	MailboxHasNoMessages   = iota
	MailboxHasMoreMessages = iota
)

