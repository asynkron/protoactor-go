package mailbox

type queue interface {
	Push(interface{})
	Pop() interface{}
}
