// package main

// import (
// 	"bufio"
// 	"fmt"
// 	"os"
// 	"time"

// 	"github.com/rogeralsing/goactor"
// )

// type Hello struct{ Who string }
// type HelloActor struct{}

// func (state *HelloActor) Receive(context actor.Context) {
// 	switch msg := context.Message().(type) {
// 	case actor.Starting:
// 		fmt.Println("Starting, initialize actor here")
// 	case actor.Stopping:
// 		fmt.Println("Stopping, actor is about shut down")
// 	case actor.Stopped:
// 		fmt.Println("Stopped, actor and it's children are stopped")
// 	case Hello:
// 		fmt.Printf("Hello %v\n", msg.Who)
// 	}
// }

// func NewHelloActor() actor.Actor {
// 	return &HelloActor{}
// }

// func main() {
// 	actor := actor.ActorOf(actor.Props(NewHelloActor))
// 	actor.Tell(Hello{Who: "Roger"})
    
//     //why wait? 
//     //Stop is a system message and is not processed through the user message mailbox
//     //thus, it will be handled _before_ any user message
// 	time.Sleep(1 * time.Second)
// 	actor.Stop()
// 	reader := bufio.NewReader(os.Stdin)
// 	reader.ReadString('\n')
// }
