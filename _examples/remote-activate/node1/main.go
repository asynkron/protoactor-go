package main

import (
	"log/slog"
	"time"

	"remoteactivate/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	timeout := 5 * time.Second

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8081)
	r := remote.NewRemote(system, remoteConfig)
	r.Start()

	rootContext := system.Root

	props := actor.
		PropsFromFunc(func(context actor.Context) {
			switch context.Message().(type) {
			case *actor.Started:
				context.Logger().Info("actor started ", slog.Any("self", context.Self()))
			case *messages.HelloRequest:
				context.Logger().Info("Received pong from sender")
				message := &messages.HelloResponse{Message: "hello from remote"}
				context.Request(context.Sender(), message)
			}
		})

	pidResp, _ := rootContext.SpawnNamed(props, "hello")

	res, _ := rootContext.RequestFuture(pidResp, &messages.HelloRequest{}, timeout).Result()

	response := res.(*messages.HelloResponse)

	system.Logger().Info("Response from remote", slog.Any("message", response.Message))

	console.ReadLine()
}
