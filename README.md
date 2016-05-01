# Go Actor Model

GAM is a MVP Actor Model framework for Go.<br/>
<br/>

Design principles:

#### Minimalistic API

In the spirit of Go, the API should be small and easy to use.
Avoid enterprisey JVM like containers and configurations.

#### Build on existing technologies

There are already a lot of great tech for e.g. networking and clustering, build on those.
e.g. gRPC streams for networking, Consul.IO for clustering.

#### Pass data, not objects

Serialization is an explicit concern, don't try to hide it.
Protobuf all the way.

#### Be fast

Do not trade performance for magic API trickery.

Ultra fast remoting, GAM currently manages to pass 800k+ messages between nodes using only two actors, while still preserving message order!

```
:> node1.exe
2016/04/30 20:33:48 Host is 127.0.0.1:55567
2016/04/30 20:33:48 Started EndpointManager
2016/04/30 20:33:48 Starting GAM server on 127.0.0.1:55567.
2016/04/30 20:33:48 Started EndpointWriter for host 127.0.0.1:8080
2016/04/30 20:33:48 Connecting to host 127.0.0.1:8080
2016/04/30 20:33:48 Connected to host 127.0.0.1:8080
2016/04/30 20:33:48 Getting stream from host 127.0.0.1:8080
2016/04/30 20:33:48 Got stream from host 127.0.0.1:8080
2016/04/30 20:33:48 Starting to send
2016/04/30 20:33:48 50000
2016/04/30 20:33:48 100000
...snip...
2016/04/30 20:33:50 950000
2016/04/30 20:33:50 1000000
2016/04/30 20:33:50 Elapsed 2.4237125s

2016/04/30 20:33:50 Msg per sec 825180 <---
```

---

## Why Actors?

![batman](/resources/batman.jpg)

* Decoupled Concurrency
* Distributed by default
* Fault tolerance

For a more indepth description of the differences, see this thread [Actors vs. CSP](https://www.quora.com/Go-programming-language-How-are-Akka-actors-are-different-than-Goroutines-and-Channels)

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

func main() {
    pid := actor.SpawnTemplate(&HelloActor{})
    pid.Tell(Hello{Who: "Roger"})
    bufio.NewReader(os.Stdin).ReadString('\n')
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
    pid := actor.Spawn(actor.Props(NewBecomeActor))
    pid.Tell(Hello{Who: "Roger"})
    pid.Tell(Hello{Who: "Roger"})
    bufio.NewReader(os.Stdin).ReadString('\n')
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
	case actor.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func NewHelloActor() actor.Actor {
	return &HelloActor{}
}

func main() {
	actor := actor.Spawn(actor.Props(NewHelloActor))
	actor.Tell(Hello{Who: "Roger"})

	//why wait?
	//Stop is a system message and is not processed through the user message mailbox
	//thus, it will be handled _before_ any user message
    //we only do this to show the correct order of events in the console
	time.Sleep(1 * time.Second)
	actor.Stop()

	bufio.NewReader(os.Stdin).ReadString('\n')
}

```

## Supervision

Root actors are supervised by the `actor.DefaultSupervisionStrategy()`, which always issues a `actor.RestartDirective` for failing actors<br/>
Child actors are supervised by their parents.<br/>
Parents can customize their child supervisor strategy using `gam.Props`<br/>
<br/>
Example<br/>
```go
type Hello struct{ Who string }
type ParentActor struct{}

func (state *ParentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		child := context.Spawn(actor.Props(NewChildActor))
		child.Tell(msg)
	}
}

func NewParentActor() actor.Actor {
	return &ParentActor{}
}

type ChildActor struct{}

func (state *ChildActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case actor.Started:
		fmt.Println("Starting, initialize actor here")
	case actor.Stopping:
		fmt.Println("Stopping, actor is about shut down")
	case actor.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
	case actor.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
		panic("Ouch")
	}
}

func NewChildActor() actor.Actor {
	return &ChildActor{}
}

func main() {
	decider := func(child *actor.PID, reason interface{}) actor.Directive {
		fmt.Println("handling failure for child")
		return actor.StopDirective
	}
	supervisor := actor.NewOneForOneStrategy(10, 1000, decider)
	pid := actor.Spawn(actor.Props(NewParentActor).WithSupervisor(supervisor))
	pid.Tell(Hello{Who: "Roger"})

	bufio.NewReader(os.Stdin).ReadString('\n')
}
```
## Networking / Remoting

GAM's networking layer is built as a thin wrapper ontop of gRPC and message serialization is built on Protocol Buffers<br/>

Example:<br/>

### Node 1
```go
type MyActor struct{
	count int
}

func (state *MyActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.Response:
		state.count++
		fmt.Println(state.count)
	}
}

func main() {
	remoting.StartServer("localhost:8090")

	pid := actor.SpawnTemplate(&MyActor{})
	message := &messages.Echo{Message: "hej", Sender: pid}
	
	//this is the remote actor we want to communicate with
	remote := actor.NewPID("localhost:8091", "myactor")
	for i := 0; i < 10; i++ {
		remote.Tell(message)
	}

	bufio.NewReader(os.Stdin).ReadString('\n')
}
```

### Node 2
```go
type MyActor struct{}

func (*MyActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.Echo:
		msg.Sender.Tell(&messages.Response{
			SomeValue: "result",
		})
	}
}

func main() {
	remoting.StartServer("localhost:8091")
	pid := actor.SpawnTemplate(&MyActor{})
	
	//register a name for our local actor so that it can be discovered remotely
	actor.ProcessRegistry.Register("myactor", pid)
	bufio.NewReader(os.Stdin).ReadString('\n')
}
```

### Message Contracts

```proto
syntax = "proto3";
package messages;
import "actor.proto"; //we need to import actor.proto, so our messages can include PID's

//this is the message the actor on node 1 will send to the remote actor on node 2
message Echo {
  actor.PID Sender = 1; //this is the PID the remote actor should reply to
  string Message = 2;
}

//this is the message the remote actor should reply with
message Response {
  string SomeValue = 1;
}
```

For more examples, see the example folder in this repository.