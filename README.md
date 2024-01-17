[![Go Report Card](https://goreportcard.com/badge/github.com/asynkron/protoactor-go)](https://goreportcard.com/report/github.com/asynkron/protoactor-go)
[![GoDoc](https://godoc.org/github.com/asynkron/protoactor-go?status.svg)](https://godoc.org/github.com/asynkron/protoactor-go)
[![checks](https://github.com/asynkron/protoactor-go/actions/workflows/checks.yml/badge.svg)](https://github.com/asynkron/protoactor-go/actions/workflows/checks.yml)
[![Sourcegraph](https://sourcegraph.com/github.com/asynkron/protoactor-go/-/badge.svg)](https://sourcegraph.com/github.com/asynkron/protoactor-go?badge)

### [Join our Slack channel](https://join.slack.com/t/asynkron/shared_invite/zt-ko824601-yGN1d3GHF9jzZX2VtONodQ)

# Cross platform actors

Introducing cross platform actor support between Go and C#.

Can I use this?
The Go implementation is still in beta, there are users using Proto Actor for Go in production already.
But be aware that the API might change over time until 1.0.

## Sourcecode - Go

This is the Go repository for Proto Actor.

The C# implementation can be found
here [https://github.com/asynkron/protoactor-dotnet](https://github.com/asynkron/protoactor-dotnet)

## Design principles:

**Minimalistic API** -
The API should be small and easy to use.
Avoid enterprisey JVM like containers and configurations.

**Build on existing technologies** - There are already a lot of great tech for e.g. networking and clustering, build on
those.
e.g. gRPC streams for networking, Consul.IO for clustering.

**Pass data, not objects** - Serialization is an explicit concern, don't try to hide it.
Protobuf all the way.

**Be fast** - Do not trade performance for magic API trickery.

Ultra fast remoting, Proto Actor currently manages to pass over **two million messages per second** between nodes using
only two actors, while still preserving message order!
This is six times more the new super advanced UDP based Artery transport for Scala Akka, and 30 times faster than
Akka.NET.

```text
:> node1.exe
Started EndpointManager
Started Activator
Starting Proto.Actor server address="127.0.0.1:8081"
Started EndpointWatcher address="127.0.0.1:8080"
Started EndpointWriter address="127.0.0.1:8080"
EndpointWriter connecting address="127.0.0.1:8080"
EndpointWriter connected address="127.0.0.1:8080"
2020/06/22 10:45:20 Starting to send
2020/06/22 10:45:20 50000
2020/06/22 10:45:20 100000
2020/06/22 10:45:20 150000
... snip ...
2020/06/22 10:45:21 900000
2020/06/22 10:45:21 950000
2020/06/22 10:45:21 1000000
2020/06/22 10:45:21 Elapsed 732.9921ms
2020/06/22 10:45:21 Msg per sec 2728542 <--

```

## History

As the creator of the Akka.NET project, I have come to some distinct conclusions while being involved in that project.
In Akka.NET we created our own thread pool, our own networking layer, our own serialization support, our own
configuration support etc. etc.
This was all fun and challenging, it is however now my firm opinion that this is the wrong way to go about things.

**If possible, software should be composed, not built**, only add code to glue existing pieces together.
This yields a much better time to market, and allows us to focus on solving the actual problem at hand, in this case
concurrency and distributed programming.

Proto Actor builds on existing technologies, Protobuf for serialization, gRPC streams for network transport.
This ensures cross platform compatibility, network protocol version tolerance and battle proven stability.

Another extremely important factor here is business agility and having an exit strategy.
By being cross platform, your organization is no longer tied into a specific platform, if you are migrating from .NET to
Go,
This can be done while still allowing actor based services to communicate between platforms.

Reinvent by not reinventing.

---

## Why Actors

![batman](/resources/batman.jpg)

- Decoupled Concurrency
- Distributed by default
- Fault tolerance

For a more indepth description of the differences, see this
thread [Actors vs. CSP](https://www.quora.com/Go-programming-language-How-are-Akka-actors-are-different-than-Goroutines-and-Channels)

## Building

You need to ensure that your `$GOPATH` variable is properly set.

Next, install the [standard protocol buffer implementation](https://github.com/google/protobuf) and run the following
commands to get all the necessary tooling:

```
go get github.com/asynkron/protoactor-go/...
cd $GOPATH/src/github.com/asynkron/protoactor-go
go get ./...
make
```

After invoking last command you will have generated protobuf definitions and built the project.

Windows users can use Cygwin to run make: [www.cygwin.com](https://www.cygwin.com/)

## Testing

This command exectutes all tests in the repository except for consul integration tests (you need consul for running
those tests). We also skip directories that don't contain any tests.

```
go test `go list ./... | grep -v "/examples/" | grep -v "/persistence" | grep -v "/scheduler"`
```
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
    context := actor.EmptyRootContext
    props := actor.PropsFromProducer(func() actor.Actor { return &HelloActor{} })
    pid, err := context.Spawn(props)
    if err != nil {
        panic(err)
    }
    context.Send(pid, Hello{Who: "Roger"})
    console.ReadLine()
}
```

## State machines / SetBehavior, PushBehavior and PopBehavior

```go
type Hello struct{ Who string }
type SetBehaviorActor struct{}

func (state *SetBehaviorActor) Receive(context actor.Context) {
    switch msg := context.Message().(type) {
    case Hello:
        fmt.Printf("Hello %v\n", msg.Who)
        context.SetBehavior(state.Other)
    }
}

func (state *SetBehaviorActor) Other(context actor.Context) {
    switch msg := context.Message().(type) {
    case Hello:
        fmt.Printf("%v, ey we are now handling messages in another behavior", msg.Who)
    }
}

func NewSetBehaviorActor() actor.Actor {
    return &SetBehaviorActor{}
}

func main() {
    context := actor.EmptyRootContext
    props := actor.PropsFromProducer(NewSetBehaviorActor)
    pid, err := context.Spawn(props)
    if err != nil {
        panic(err)
    }
    context.Send(pid, Hello{Who: "Roger"})
    context.Send(pid, Hello{Who: "Roger"})
    console.ReadLine()
}
```

## Lifecycle events

Unlike Akka, Proto Actor uses messages for lifecycle events instead of OOP method overrides

```go
type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context actor.Context) {
    switch msg := context.Message().(type) {
    case *actor.Started:
        fmt.Println("Started, initialize actor here")
    case *actor.Stopping:
        fmt.Println("Stopping, actor is about shut down")
    case *actor.Stopped:
        fmt.Println("Stopped, actor and its children are stopped")
    case *actor.Restarting:
        fmt.Println("Restarting, actor is about restart")
    case Hello:
        fmt.Printf("Hello %v\n", msg.Who)
    }
}

func main() {
    context := actor.EmptyRootContext
    props := actor.PropsFromProducer(func() actor.Actor { return &HelloActor{} })
    pid, err := context.Spawn(props)
    if err != nil {
        panic(err)
    }
    context.Send(pid, Hello{Who: "Roger"})

    // why wait?
    // Stop is a system message and is not processed through the user message mailbox
    // thus, it will be handled _before_ any user message
    // we only do this to show the correct order of events in the console
    time.Sleep(1 * time.Second)
    context.Stop(pid)

    console.ReadLine()
}
```

## Supervision

Root actors are supervised by the `actor.DefaultSupervisionStrategy()`, which always issues a `actor.RestartDirective`
for failing actors
Child actors are supervised by their parents.
Parents can customize their child supervisor strategy using `Proto Actor.Props`

### Example

```go
type Hello struct{ Who string }
type ParentActor struct{}

func (state *ParentActor) Receive(context actor.Context) {
    switch msg := context.Message().(type) {
    case Hello:
        props := actor.PropsFromProducer(NewChildActor)
        child := context.Spawn(props)
        context.Send(child, msg)
    }
}

func NewParentActor() actor.Actor {
    return &ParentActor{}
}

type ChildActor struct{}

func (state *ChildActor) Receive(context actor.Context) {
    switch msg := context.Message().(type) {
    case *actor.Started:
        fmt.Println("Starting, initialize actor here")
    case *actor.Stopping:
        fmt.Println("Stopping, actor is about shut down")
    case *actor.Stopped:
        fmt.Println("Stopped, actor and its children are stopped")
    case *actor.Restarting:
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
	decider := func(reason interface{}) actor.Directive {
		log.Printf("handling failure for child. reason:%v", reason)

		// return actor.StopDirective
		return actor.RestartDirective
	}
	supervisor := actor.NewOneForOneStrategy(10, 1000, decider)

	ctx := actor.NewActorSystem().Root
	props := actor.PropsFromProducer(NewParentActor).WithSupervisor(supervisor)

	pid := ctx.Spawn(props)
	ctx.Send(pid, Hello{Who: "Roger"})

	console.ReadLine()
}
```

## Networking / Remoting

Proto Actor's networking layer is built as a thin wrapper ontop of gRPC and message serialization is built on Protocol
Buffers<br/>

### Example

#### Node 1

```go
type MyActor struct {
    count int
}

func (state *MyActor) Receive(context actor.Context) {
    switch context.Message().(type) {
    case *messages.Response:
        state.count++
        fmt.Println(state.count)
    }
}

func main() {
    remote.Start("localhost:8090")

    context := actor.EmptyRootContext
    props := actor.PropsFromProducer(func() actor.Actor { return &MyActor{} })
    pid, _ := context.Spawn(props)
    message := &messages.Echo{Message: "hej", Sender: pid}

    // this is to spawn remote actor we want to communicate with
    spawnResponse, _ := remote.SpawnNamed("localhost:8091", "myactor", "hello", time.Second)

    // get spawned PID
    spawnedPID := spawnResponse.Pid
    for i := 0; i < 10; i++ {
        context.Send(spawnedPID, message)
    }

    console.ReadLine()
}
```

#### Node 2

```go
type MyActor struct{}

func (*MyActor) Receive(context actor.Context) {
    switch msg := context.Message().(type) {
    case *messages.Echo:
        context.Send(msg.Sender, &messages.Response{
            SomeValue: "result",
        })
    }
}

func main() {
    remote.Start("localhost:8091")

    // register a name for our local actor so that it can be spawned remotely
    remote.Register("hello", actor.PropsFromProducer(func() actor.Actor { return &MyActor{} }))
    console.ReadLine()
}
```

### Message Contracts

```proto
syntax = "proto3";
package messages;
import "actor.proto"; // we need to import actor.proto, so our messages can include PID's

// this is the message the actor on node 1 will send to the remote actor on node 2
message Echo {
  actor.PID Sender = 1; // this is the PID the remote actor should reply to
  string Message = 2;
}

// this is the message the remote actor should reply with
message Response {
  string SomeValue = 1;
}
```

Notice: always use "gogoslick_out" instead of "go_out" when generating proto code. "gogoslick_out" will create type
names which will be used during serialization.

For more examples, see the example folder in this repository.

## Contributors

<a href="https://github.com/asynkron/protoactor-go/graphs/contributors">
  <img src="https://contributors-img.web.app/image?repo=asynkron/protoactor-go" />
</a>

Made with [contributors-img](https://contributors-img.web.app). 
