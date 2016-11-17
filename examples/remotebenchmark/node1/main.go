package main

import (
	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting"

	"log"
	"sync"

	"time"
)

// import "runtime/pprof"
import "github.com/AsynkronIT/gam/examples/remotebenchmark/messages"
import "runtime"

type localActor struct {
	count        int
	wgStart      *sync.WaitGroup
	wgStop       *sync.WaitGroup
	start        time.Time
	messageCount int
}

func (state *localActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *actor.Started:
		state.wgStart.Add(1)
		state.wgStop.Add(1)
	case *messages.Pong:
		state.count++
		if state.count%50000 == 0 {
			log.Println(state.count)
		}
		if state.count == state.messageCount {
			state.wgStop.Done()
		}
	case *messages.Start:
		state.wgStart.Done()
	}
}

type FakeMessage struct {
	pid *actor.PID
}

func newLocalActor(start *sync.WaitGroup, stop *sync.WaitGroup, messageCount int) actor.Producer {
	return func() actor.Actor {
		return &localActor{
			wgStart:      start,
			wgStop:       stop,
			messageCount: messageCount,
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var wgStop sync.WaitGroup
	var wgStart sync.WaitGroup

	messageCount := 1000000

	remoting.Start("127.0.0.1:8081")

	props := actor.
		FromProducer(newLocalActor(&wgStart, &wgStop, messageCount)).
		WithMailbox(actor.NewBoundedMailbox(1000, 10000))

	pid := actor.Spawn(props)

	remote := actor.NewPID("127.0.0.1:8080", "remote")
	remote.Ask(&messages.StartRemote{}, pid)

	wgStart.Wait()
	start := time.Now()
	log.Println("Starting to send")

	message := &messages.Ping{}
	for i := 0; i < messageCount; i++ {
		remote.Ask(message, pid)
	}

	wgStop.Wait()
	elapsed := time.Since(start)
	log.Printf("Elapsed %s", elapsed)

	x := int(float32(messageCount*2) / (float32(elapsed) / float32(time.Second)))
	log.Printf("Msg per sec %v", x)

	// f, err := os.Create("memprofile")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.WriteHeapProfile(f)
	// f.Close()
}
