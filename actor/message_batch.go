package actor

// MessageBatch represents a collection of messages delivered together.
type MessageBatch interface {
	GetMessages() []interface{}
}
