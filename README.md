# Go Actor Model

GAM is a MVP port of JVM Akka.Actor to Go.

## Design philosophy:
 
* Do one thing only, Actors
* Networking and Clustering should be solved using other tools, e.g. gRPC and Consul
* Serialization should be an external concern, GAM infrastructure and primitives should not be serialized

## Hello world
```go
type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func NewHelloActor() gam.Actor {
	return &HelloActor{}
}

func main() {
	actor := gam.ActorOf(actor.Props(NewHelloActor))
	actor.Tell(Hello{Who: "Roger"})

  ...
}
```

## State machines / Become and Unbecome

```go
type Become struct {}
type Hello struct{ Who string }
type BecomeActor struct{}

func (state *BecomeActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
        context.Become(state.Other)
	}
}

func (state *BecomeActor) Other(context gam.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("%v, ey we are now handling messages in another behavior",msg.Who)
	}
}

func NewBecomeActor() gam.Actor {
	return &BecomeActor{}
}

func main() {
	actor := gam.ActorOf(actor.Props(NewBecomeActor))
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

func (state *HelloActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {
	case gam.Started:
		fmt.Println("Started, initialize actor here")
	case gam.Stopping:
		fmt.Println("Stopping, actor is about shut down")
	case gam.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
	case gam.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func NewHelloActor() gam.Actor {
	return &HelloActor{}
}

func main() {
	actor := gam.ActorOf(actor.Props(NewHelloActor))
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

Root actors are supervised by the `actor.DefaultSupervisionStrategy()`, which always issues a `actor.RestartDirective` for failing actors

```go
type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {
	case gam.Started:
		fmt.Println("Starting, initialize actor here")
	case gam.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
        panic("Ouch")
	}
}

func NewHelloActor() gam.Actor {
	return &HelloActor{}
}

func main() {
	actor := gam.ActorOf(actor.Props(NewHelloActor))
	actor.Tell(Hello{Who: "Roger"})
	
  ...
}
```

Child actors are supervised by their parents.
Parents can customize their child supervisor strategy using `gam.Props`

```go
decider := func(child gam.ActorRef, reason interface{}) gam.Directive {
	fmt.Println("handling failure for child")
	return gam.StopDirective
}
supervisor := gam.NewOneForOneStrategy(10,1000,decider)
actor := gam.ActorOf(gam.Props(NewParentActor).WithSupervisor(supervisor))
```

Example
```go
type Hello struct{ Who string }
type ParentActor struct{}

func (state *ParentActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {	
	case Hello:
		child := context.ActorOf(gam.Props(NewChildActor))
		child.Tell(msg)
	}
}

func NewParentActor() gam.Actor {
	return &ParentActor{}
}

type ChildActor struct{}

func (state *ChildActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {
	case gam.Started:
		fmt.Println("Starting, initialize actor here")
	case gam.Stopping:
		fmt.Println("Stopping, actor is about shut down")
	case gam.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
	case gam.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
        panic("Ouch")
	}
}

func NewChildActor() gam.Actor {
	return &ChildActor{}
}

func main() {
	decider := func(child gam.ActorRef, reason interface{}) gam.Directive {
		fmt.Println("handling failure for child")
		return gam.StopDirective
	}
	supervisor := gam.NewOneForOneStrategy(10,1000,decider)
	actor := gam.ActorOf(gam.Props(NewParentActor).WithSupervisor(supervisor))
	actor.Tell(Hello{Who: "Roger"})
	
	...
}
```