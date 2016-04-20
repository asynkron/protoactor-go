package gam

// import (
// 	"bufio"
// 	"fmt"
// 	"os"
// )
// import "github.com/rogeralsing/goactor/actor"

// func main() {
// 	props := actor.
// 		Props(NewParentActor).
// 		WithMailbox(actor.NewUnboundedMailbox()).
// 		WithSupervisor(actor.DefaultStrategy())

// 	parent := actor.ActorOf(props)
// 	parent.Tell(Hello{Name: "Roger"})
// 	parent.Tell(Hello{Name: "Go"})

// 	reader := bufio.NewReader(os.Stdin)
// 	reader.ReadString('\n')
// }

// //Messages
// type Ping struct{ Sender actor.ActorRef }
// type Pong struct{}
// type Hello struct{ Name string }

// //Child actor
// type ChildActor struct{ messageCount int }

// func NewChildActor() actor.Actor {
// 	return &ChildActor{}
// }

// func (state *ChildActor) Receive(context actor.Context) {
// 	switch msg := context.Message().(type) {
// 	case Ping:
// 		state.messageCount++
// 		fmt.Printf("message count %v \n", state.messageCount)
// 		msg.Sender.Tell(Pong{})
// 	}
// }

// //Parent actor
// type ParentActor struct{ child actor.ActorRef }

// func NewParentActor() actor.Actor {
// 	return &ParentActor{}
// }

// func (state *ParentActor) Receive(context actor.Context) {
// 	switch msg := context.Message().(type) {
// 	case actor.Starting:
// 		state.child = context.ActorOf(actor.Props(NewChildActor))
// 	case Hello:
// 		fmt.Printf("Parent got hello %v\n", msg.Name)
// 		state.child.Tell(Ping{Sender: context.Self()})
// 	}
// }
