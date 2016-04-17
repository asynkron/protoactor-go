package actor

type Context struct {
	*ActorCell
    Message interface{}	
}

func NewContext(cell *ActorCell,message interface{}) *Context {
    return &Context {
        ActorCell: cell,
        Message: message,
    }
}