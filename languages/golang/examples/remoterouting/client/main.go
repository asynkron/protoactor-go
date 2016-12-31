package main

import (
	"log"
	"runtime"

	"sync"

	"fmt"
	"time"

	"github.com/AsynkronIT/protoactor/languages/golang/examples/remoterouting/messages"
	"github.com/AsynkronIT/protoactor/languages/golang/src/actor"
	"github.com/AsynkronIT/protoactor/languages/golang/src/remoting"
	"github.com/AsynkronIT/protoactor/languages/golang/src/routing"

	console "github.com/AsynkronIT/goconsole"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	remoting.Start("127.0.0.1:8100")

	p1 := actor.NewPID("127.0.0.1:8101", "remote")
	p2 := actor.NewPID("127.0.0.1:8102", "remote")
	router := routing.NewConsistentHashGroup(p1, p2)
	props := actor.FromGroupRouter(router)

	remote := actor.Spawn(props)

	messageCount := 1000000

	var wgStop sync.WaitGroup

	props = actor.
		FromProducer(newLocalActor(&wgStop, messageCount)).
		WithMailbox(actor.NewBoundedMailbox(10000))

	pid := actor.Spawn(props)

	log.Println("Starting to send")

	t := time.Now()

	for i := 0; i < messageCount; i++ {
		message := &messages.Ping{User: fmt.Sprintf("User_%d", i)}
		remote.Request(message, pid)
	}

	wgStop.Wait()

	pid.Stop()

	fmt.Printf("elapsed: %v\n", time.Since(t))

	console.ReadLine()
}
