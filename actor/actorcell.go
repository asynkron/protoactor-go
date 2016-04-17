package actor
import "fmt"

type ActorCell struct {
	actor           Actor
}

func (cell *ActorCell) invokeSystemMessage(message interface{}) {
	fmt.Printf("Received system message %v\n", message)
}

func (cell *ActorCell) invokeUserMessage(message interface{}) {
	cell.actor.Receive(message)
}