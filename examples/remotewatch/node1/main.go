package main

import (
	"log"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/remote"
)

func main() {
	timeout := 5 * time.Second
	remote.Start("127.0.0.1:8081")

	context := actor.EmptyRootContext
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			log.Println("Local actor started")
			pidResp, err := remote.SpawnNamed("127.0.0.1:8080", "myRemote", "remote", timeout)
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
