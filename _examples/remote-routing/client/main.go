package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"remoterouting/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/asynkron/protoactor-go/router"
)

var (
	system      = actor.NewActorSystem()
	rootContext = system.Root
)

func main() {
	cfg := remote.Configure("127.0.0.1", 8100)
	r := remote.NewRemote(system, cfg)
	r.Start()

	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	p1 := actor.NewPID("127.0.0.1:8101", "remote")
	p2 := actor.NewPID("127.0.0.1:8102", "remote")

	remotePID := rootContext.Spawn(router.NewConsistentHashGroup(p1, p2))

	messageCount := 1000000

	var wgStop sync.WaitGroup

	props := actor.
		PropsFromProducer(newLocalActor(&wgStop, messageCount),
			actor.WithMailbox(actor.Bounded(10000)))

	pid := rootContext.Spawn(props)

	log.Println("Starting to send")

	t := time.Now()

	for i := 0; i < messageCount; i++ {
		message := &messages.Ping{User: fmt.Sprintf("User_%d", i)}
		rootContext.RequestWithCustomSender(remotePID, message, pid)
	}

	wgStop.Wait()

	rootContext.Stop(pid)

	fmt.Printf("elapsed: %v\n", time.Since(t))

	console.ReadLine()
}
