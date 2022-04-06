package actor

type MessageBatch interface {
	GetMessages() []any
}
