package actor

type MailboxQueue interface {
	Push(interface{})
	Pop() interface{}
}
