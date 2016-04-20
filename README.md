# Go Actor Model

GAM is a MVP port of JVM Akka.Actor to Go.

## Hello world
```go
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

  ...
}
```

## State machines / Become and Unbecome

```go
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
  
  ...  
}
```

## Lifecycle events
Unlike Akka, GAM uses messages for lifecycle events instead of OOP method overrides

```go
type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case actor.Started:
		fmt.Println("Started, initialize actor here")
	case actor.Stopping:
		fmt.Println("Stopping, actor is about shut down")
	case actor.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
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
    
    //why wait? 
    //Stop is a system message and is not processed through the user message mailbox
    //thus, it will be handled _before_ any user message
	time.Sleep(1 * time.Second)
	actor.Stop()

  ...
}

```

## Supervision

```go
type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case actor.Started:
		fmt.Println("Starting, initialize actor here")
	case actor.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
        panic("Ouch")
	}
}

func NewHelloActor() actor.Actor {
	return &HelloActor{}
}

func main() {
	actor := actor.ActorOf(actor.Props(NewHelloActor))
	actor.Tell(Hello{Who: "Roger"})
	
  ...
}
```