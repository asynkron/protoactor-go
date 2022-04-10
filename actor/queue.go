package actor

type queue interface {
	Push(interface{})
	Pop() interface{}
}
