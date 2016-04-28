package main

import (
	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/remoting"

	"log"
	"sync"

	"time"
)

// import "runtime/pprof"
import "github.com/rogeralsing/gam/examples/remoting/messages"
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

func main() {
	runtime.GOMAXPROCS(8)
	// f, err := os.Create("cpuprofile")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()
	var wgStop sync.WaitGroup
	wgStop.Add(1)
	var wgStart sync.WaitGroup
	wgStart.Add(1)

	messageCount := 1000000

	remoting.StartServer("localhost:8090")

	pid := actor.SpawnTemplate(&localActor{
		wgStart:      &wgStart,
		wgStop:       &wgStop,
		messageCount: messageCount,
	})

	message := &messages.Ping{Sender: pid}
	remote := actor.NewPID("localhost:8091", "remote")
	remote.Tell(&messages.StartRemote{Sender: pid})
	wgStart.Wait()
	log.Println("Starting to send")
	start := time.Now()
	for i := 0; i < messageCount; i++ {
		remote.Tell(message)
	}

	wgStop.Wait()
	elapsed := time.Since(start)
	log.Printf("Elapsed %s", elapsed)

	x := int(float32(messageCount*2) / (float32(elapsed) / float32(time.Second)))
	log.Printf("Msg per sec %v", x)
}
