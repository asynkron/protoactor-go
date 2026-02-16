package actor

type queue interface {
	Push(any)
	Pop() any
}
