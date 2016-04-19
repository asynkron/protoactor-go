package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
)

type ContextValue struct {
	*ActorCell
	message interface{}
}

func (context *ContextValue) Message() interface{} {
	return context.message
}

func NewContext(cell *ActorCell, message interface{}) interfaces.Context {
	res := &ContextValue{
		ActorCell: cell,
		message:   message,
	}
	return res
}
