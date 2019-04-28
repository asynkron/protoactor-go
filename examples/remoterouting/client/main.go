package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/remoterouting/messages"
	"github.com/AsynkronIT/protoactor-go/mailbox"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/AsynkronIT/protoactor-go/router"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	remote.Start("127.0.0.1:8100")

	p1 := actor.NewPID("127.0.0.1:8101", "remote")
	p2 := actor.NewPID("127.0.0.1:8102", "remote")

	rootContext := actor.EmptyRootContext

	remotePID := rootContext.Spawn(router.NewConsistentHashGroup(p1, p2))

	messageCount := 1000000

	var wgStop sync.WaitGroup

	props := actor.
		PropsFromProducer(newLocalActor(&wgStop, messageCount)).
		WithMailbox(mailbox.Bounded(10000))

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
