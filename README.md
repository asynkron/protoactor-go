# Go Actor Model

GAM is a MVP port of JVM Akka.Actor to Go.

```go

type Hello struct { Who string }

func (state *HelloActor) Receive(context Context) {
  switch msg := context.Message().(type) {
     case Hello:
         fmt.Printf("Hello %v",msg.Who)
  }
}

func NewHelloActor() Actor {
	return &HelloActor{}
}



func main() {
	actor := actor.ActorOf(actor.Props(NewHelloActor))
	actor.Tell(Hello{Who:"Roger"})
}
```
