package main

import (
	"log"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func main() {
	timeout := 5 * time.Second
	remote.Start("127.0.0.1:8081")

	props := actor.FromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			log.Println("Local actor started")
			pid, err := remote.SpawnNamed("127.0.0.1:8080", "myRemote", "remote", timeout)
			if err != nil {
				log.Print("Local failed to spawn remote actor")
				return
			}
			log.Println("Local spawned remote actor")
			ctx.Watch(pid)
			log.Println("Local is watching remote actor")
		case *actor.Terminated:
			log.Printf("Local got terminated message %+v", msg)
		}
	})
	actor.Spawn(props)
	console.ReadLine()
}
