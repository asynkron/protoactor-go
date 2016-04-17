package actor
import "fmt"

type ActorCell struct {
    self            ActorRef
	actor           Actor
}

func (cell *ActorCell) invokeSystemMessage(message interface{}) {
	fmt.Printf("Received system message %v\n", message)
}

func (cell *ActorCell) invokeUserMessage(message interface{}) {
    context := MessageContext {
        Self: cell.self,
        Message: message,
    }
	cell.actor.Receive(&context)
}