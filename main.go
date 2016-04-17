package main

import "fmt"
import "bufio"
import "os"
import "github.com/rogeralsing/goactor/actor"

func main() {
	myActor := actor.ActorOf(new(MyActor))
	myActor.Tell(Hello{Name: "Roger"})
	myActor.Tell(Hello{Name: "Go"})
	bufio.NewReader(os.Stdin).ReadString('\n')
}

type MyActor struct{ messageCount int }
type Hello struct{ Name string }

func (state *MyActor) Receive(message interface{}) {
	switch msg := message.(type) {
	default:
		fmt.Printf("unexpected type %T\n", msg) // %T prints whatever type t has
	case Hello:
		fmt.Printf("Hello %v\n", msg.Name) // t has type bool
		state.messageCount++
	}
}
