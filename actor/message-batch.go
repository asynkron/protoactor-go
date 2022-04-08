package actor

type MessageBatch interface {
	GetMessages() []interface{}
}
