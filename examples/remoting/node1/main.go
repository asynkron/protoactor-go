package main

import "github.com/rogeralsing/gam/actor"
import "github.com/rogeralsing/gam/remoting"

import "log"
import "sync"

import "time"

// import "runtime/pprof"
import "github.com/rogeralsing/gam/examples/remoting/messages"
import "runtime"

type localActor struct {
	count        int
	wg           *sync.WaitGroup
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
		if state.count == 1 {
			state.start = time.Now()
		}
		if state.count == state.messageCount {
			elapsed := time.Since(state.start)
			log.Printf("Elapsed %s", elapsed)

			x := int(float32(state.messageCount*2) / (float32(elapsed) / float32(time.Second)))
			log.Printf("Msg per sec %v", x)

			state.wg.Done()
		}
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
	var wg sync.WaitGroup
	wg.Add(1)

	messageCount := 1000000
	fillers := 10

	remoting.StartServer("localhost:8090")

	pid := actor.SpawnTemplate(&localActor{wg: &wg, messageCount: messageCount})

	message := &messages.Ping{Sender: pid}
	remote := actor.NewPID("localhost:8091", "remote")

	for j := 0; j < fillers; j++ {
		go func() {
			for i := 0; i < messageCount/fillers; i++ {
				remote.Tell(message)
			}
		}()
	}
	wg.Wait()
}
