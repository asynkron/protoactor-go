package main

import (
	"log"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	system  = actor.NewActorSystem()
	context = system.Root
)

func main() {
	cfg := remote.Configure("127.0.0.1", 8081)
	r := remote.NewRemote(system, cfg)
	r.Start()

	timeout := 5 * time.Second

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			log.Println("Local actor started")
			pidResp, err := r.SpawnNamed("127.0.0.1:8080", "myRemote", "remote", timeout)
			if err != nil {
				log.Print("Local failed to spawn remote actor")
				return
			}
			log.Println("Local spawned remote actor")
			ctx.Watch(pidResp.Pid)
			log.Println("Local is watching remote actor")
		case *actor.Terminated:
			log.Printf("Local got terminated message %+v", msg)
		}
	})

	context.Spawn(props)
	console.ReadLine()
}
