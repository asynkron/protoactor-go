package actor

type Actor interface {
	Receive(message *Context)
}