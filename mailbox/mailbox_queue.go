package mailbox

type MailboxQueue interface {
	Push(interface{})
	Pop() interface{}
}
