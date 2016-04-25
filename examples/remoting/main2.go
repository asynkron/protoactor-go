package main

import "github.com/rogeralsing/gam"
import "github.com/rogeralsing/gam/remoting"
import "bufio"
import "os"
import "github.com/rogeralsing/example/messages"

type MyActor struct{}

func (*MyActor) Receive(context gam.Context) {
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
	pid := gam.SpawnTemplate(&MyActor{})
    gam.GlobalProcessRegistry.Register("foo",pid)
	bufio.NewReader(os.Stdin).ReadString('\n')
}
