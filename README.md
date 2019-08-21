[![Go Report Card](https://goreportcard.com/badge/github.com/AsynkronIT/protoactor-go)](https://goreportcard.com/report/github.com/AsynkronIT/protoactor-go) 
[![GoDoc](https://godoc.org/github.com/AsynkronIT/protoactor-go?status.svg)](https://godoc.org/github.com/AsynkronIT/protoactor-go)
[![Build Status](https://travis-ci.org/AsynkronIT/protoactor-go.svg?branch=dev)](https://travis-ci.org/AsynkronIT/protoactor-go)
[![Coverage Status](https://coveralls.io/repos/github/AsynkronIT/protoactor-go/badge.svg?branch=dev)](https://coveralls.io/github/AsynkronIT/protoactor-go?branch=dev)
[![Sourcegraph](https://sourcegraph.com/github.com/AsynkronIT/protoactor-go/-/badge.svg)](https://sourcegraph.com/github.com/AsynkronIT/protoactor-go?badge)

[![Join the chat at https://gitter.im/AsynkronIT/protoactor](https://badges.gitter.im/AsynkronIT/protoactor.svg)](https://gitter.im/AsynkronIT/protoactor?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

# Cross platform actors

Introducing cross platform actor support between Go and C#.

Can I use this?
The Go implementation is still in beta, there are users using Proto Actor for Go in production already.
But be aware that the API might change over time until 1.0.

## Sourcecode - Go
This is the Go repository for Proto Actor.

The C# implementation can be found here [https://github.com/AsynkronIT/protoactor-dotnet](https://github.com/AsynkronIT/protoactor-dotnet)

## Design principles:

**Minimalistic API** -
The API should be small and easy to use.
Avoid enterprisey JVM like containers and configurations.

**Build on existing technologies** - There are already a lot of great tech for e.g. networking and clustering, build on those.
e.g. gRPC streams for networking, Consul.IO for clustering.

**Pass data, not objects** - Serialization is an explicit concern, don't try to hide it.
Protobuf all the way.

**Be fast** - Do not trade performance for magic API trickery.

Ultra fast remoting, Proto Actor currently manages to pass over **two million messages per second** between nodes using only two actors, while still preserving message order!
This is six times more the new super advanced UDP based Artery transport for Scala Akka, and 30 times faster than Akka.NET.

```text
:> node1.exe
2016/12/02 14:30:09 50000
2016/12/02 14:30:09 100000
2016/12/02 14:30:09 150000
... snip ...
2016/12/02 14:30:09 900000
2016/12/02 14:30:09 950000
2016/12/02 14:30:10 1000000
2016/12/02 14:30:10 Elapsed 999.9985ms
2016/12/02 14:30:10 Msg per sec 2000003 <--
```

## History

As the creator of the Akka.NET project, I have come to some distinct conclusions while being involved in that project.
In Akka.NET we created our own thread pool, our own networking layer, our own serialization support, our own configuration support etc. etc.
This was all fun and challenging, it is however now my firm opinion that this is the wrong way to go about things.

**If possible, software should be composed, not built**, only add code to glue existing pieces together.
This yields a much better time to market, and allows us to focus on solving the actual problem at hand, in this case concurrency and distributed programming.

Proto Actor builds on existing technologies, Protobuf for serialization, gRPC streams for network transport.
This ensures cross platform compatibility, network protocol version tolerance and battle proven stability.

Another extremely important factor here is business agility and having an exit strategy.
By being cross platform, your organization is no longer tied into a specific platform, if you are migrating from .NET to Go, 
This can be done while still allowing actor based services to communicate between platforms.

Reinvent by not reinventing.

---

## Why Actors

![batman](/resources/batman.jpg)

* Decoupled Concurrency
* Distributed by default
* Fault tolerance

For a more indepth description of the differences, see this thread [Actors vs. CSP](https://www.quora.com/Go-programming-language-How-are-Akka-actors-are-different-than-Goroutines-and-Channels)

## Building

You need to ensure that your `$GOPATH` variable is properly set.

Next, install the [standard protocol buffer implementation](https://github.com/google/protobuf) and run the following commands to get all the necessary tooling:

```
go get github.com/AsynkronIT/protoactor-go/...
cd $GOPATH/src/github.com/AsynkronIT/protoactor-go
go get ./...
make
```

After invoking last command you will have generated protobuf definitions and built the project.

Windows users can use Cygwin to run make: [www.cygwin.com](https://www.cygwin.com/)

## Testing

This command exectutes all tests in the repository except for consul integration tests (you need consul for running those tests). We also skip directories that don't contain any tests.

```
go test `go list ./... | grep -v consul` | grep -v 'no test files'
```

If everything is ok, you will get the output:

```
ok  	github.com/AsynkronIT/protoactor-go/actor	0.115s
ok  	github.com/AsynkronIT/protoactor-go/eventstream	0.020s
ok  	github.com/AsynkronIT/protoactor-go/internal/queue/goring	2.524s
ok  	github.com/AsynkronIT/protoactor-go/internal/queue/mpsc	2.385s
ok  	github.com/AsynkronIT/protoactor-go/log	0.017s
ok  	github.com/AsynkronIT/protoactor-go/mailbox	2.742s
ok  	github.com/AsynkronIT/protoactor-go/plugin	1.227s
ok  	github.com/AsynkronIT/protoactor-go/router	1.836s
ok  	github.com/AsynkronIT/protoactor-go/stream	0.017s
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

Root actors are supervised by the `actor.DefaultSupervisionStrategy()`, which always issues a `actor.RestartDirective` for failing actors
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
        fmt.Println("handling failure for child")
        return actor.StopDirective
    }
    supervisor := actor.NewOneForOneStrategy(10, 1000, decider)

    context := actor.EmptyRootContext
    props := actor.
        FromProducer(NewParentActor).
        WithSupervisor(supervisor)

    pid, err := context.Spawn(props)
    if err != nil {
        panic(err)
    }
    context.Send(pid, Hello{Who: "Roger"})

    console.ReadLine()
}
```

## Networking / Remoting

Proto Actor's networking layer is built as a thin wrapper ontop of gRPC and message serialization is built on Protocol Buffers<br/>

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
Notice: always use "gogoslick_out" instead of "go_out" when generating proto code. "gogoslick_out" will create type names which will be used during serialization.

For more examples, see the example folder in this repository.

### Support

Many thanks to [JetBrains](https://www.jetbrains.com) for support!
