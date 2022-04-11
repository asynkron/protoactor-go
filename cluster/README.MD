# Proto.Actor Cluster - Virtual Actors (Alpha)

## Massively distributed actors for GO

Proto.Actor supports the classic actor model also found in Erlang and Akka.<br>
Our cluster support however uses a different approach, **Virtual Actor Model**.

This is a model where each actor appears to *always exist*.
There is no lifecycle as in the classic actor model.
You get a reference to the actor by asking for it's ID.

e.g.

```go
hello := shared.GetHelloGrain("abc")
res := hello.SayHello(&shared.HelloRequest{Name: "Proto.Actor"})
```

This will ask the cluster where the 'abc' actor is located.
If it does not yet exist, it will be created for you.

See Microsoft Orleans for more info about the Virtual Actor Model:
[http://dotnet.github.io/orleans/](http://dotnet.github.io/orleans/)

## How to

## Protobuf IDL Definition

Start by defining your messages and grain contracts.
You do this by using Protobuf IDL files.

Here is the definition from the `/examples/cluster/shared` example

```proto
syntax = "proto3";
package shared;

message HelloRequest {
  string name = 1;
}

message HelloResponse {
  string message = 1;
}

message AddRequest {
  double a = 1;
  double b = 2;
}

message AddResponse {
  double result = 1;
}

service Hello {
  rpc SayHello (HelloRequest) returns (HelloResponse) {} 
  rpc Add(AddRequest) returns (AddResponse) {}
}
```

Once you have this, you can generate your code using the protobuf `protoc` compiler.

**Windows**

```batch
#generate messages
protoc -I=. -I=%GOPATH%\src --gogoslick_out=. protos.proto
#generate grains 
protoc -I=. -I=%GOPATH%\src --gorleans_out=. protos.proto 
```

## Implementing

Once the messages and contracts have been generated, you can start implementing your own business logic.
This is essentially a type which is powered by a Proto.Actor actor behind the scenes.

```go
package shared

// a Go struct implementing the Hello interface
type hello struct {
}

func (*hello) SayHello(r *HelloRequest) *HelloResponse {
	return &HelloResponse{Message: "hello " + r.Name}
}

func (*hello) Add(r *AddRequest) *AddResponse {
	return &AddResponse{Result: r.A + r.B}
}

// Register what implementation Proto.Actor should use when 
// creating actors for a certain grain type.
func init() {
	// apply DI and setup logic
	HelloFactory(func() Hello { return &hello{} })
}
```

## Seed nodes

```go
func main() {
    cluster.Start("127.0.0.1:7711")
    console.ReadLine()
}
```

## Member nodes

```go
func main() {
	cluster.Start("127.0.0.1:0", "127.0.0.1:7711")

    // get a reference to the virtual actor called "abc" of type Hello
	hello := shared.GetHelloGrain("abc")
	res := hello.SayHello(&shared.HelloRequest{Name: "Proto.Actor"})
	log.Printf("Message from grain %v", res.Message)
}
```

## FAQ

### Can I use Proto.Actor Cluster in production?

The Proto.Actor Cluster support is in alpha version, thus not production ready.

### What about performance?

Proto.Actor Remoting is able to pass 1 million+ messages per second on a standard dev machine.
This is the same infrastructure used in Proto.Actor cluster.
Proto.Actor Cluster however uses an RPC API, meaning it is Request/Response in nature.
If you wait for a response for each call, the throughput will ofcourse be a lot less.
Async Fire and forget for performance, Request/Response for simplicity.
