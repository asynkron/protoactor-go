package main

import "github.com/rogeralsing/gam/actor"
import "github.com/rogeralsing/gam/remoting"
import "bufio"
import "os"
import "github.com/rogeralsing/gam/examples/remoting/messages"

type MyActor struct{}

func (*MyActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.Echo:
        msg.Sender.Tell(&messages.Response {
            SomeValue: "result",
            AnInt: 123,
        })
	}
}

func main() {
	remoting.StartServer("localhost:8091")
	pid := actor.SpawnTemplate(&MyActor{})
    actor.GlobalProcessRegistry.Register("foo",pid)
	bufio.NewReader(os.Stdin).ReadString('\n')
}
