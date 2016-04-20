# Go Actor Model

GAM is a MVP port of JVM Akka.Actor to Go.

## Hello world
```go
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/rogeralsing/goactor"
)

type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func NewHelloActor() actor.Actor {
	return &HelloActor{}
}

func main() {
	actor := actor.ActorOf(actor.Props(NewHelloActor))
	actor.Tell(Hello{Who: "Roger"})
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
```

## State machines / Become and Unbecome

```go
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/rogeralsing/goactor"
)

type Become struct {}
type Hello struct{ Who string }
type BecomeActor struct{}

func (state *BecomeActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
        context.Become(state.Other)
	}
}

func (state *BecomeActor) Other(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("%v, ey we are now handling messages in another behavior",msg.Who)
	}
}

func NewBecomeActor() actor.Actor {
	return &BecomeActor{}
}

func main() {
	actor := actor.ActorOf(actor.Props(NewBecomeActor))
	actor.Tell(Hello{Who: "Roger"})
    actor.Tell(Hello{Who: "Roger"})
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
```