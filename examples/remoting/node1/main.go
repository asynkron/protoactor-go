package main

import "github.com/rogeralsing/gam/actor"
import "github.com/rogeralsing/gam/remoting"

import "log"
import "sync"

import "time"

// import "runtime/pprof"
import "github.com/rogeralsing/gam/examples/remoting/messages"
import "runtime"

type MyActor struct {
	count int
	wg    *sync.WaitGroup
	start time.Time
}

func (state *MyActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Response:
		state.count++
		if state.count%10000 == 0 {
			log.Println(state.count)
		}
		if state.count == 1 {
			state.start = time.Now()
		}
		if state.count == 1000000 {
			elapsed := time.Since(state.start)
			log.Printf("Elapsed %s", elapsed)
			
			x := int(2000000.0 / (float32(elapsed) / float32(time.Second)))
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

	remoting.StartServer("localhost:8090")

	pid := actor.SpawnTemplate(&MyActor{wg: &wg})

	message := &messages.Echo{Message: "", Sender: pid}
	remote := actor.NewPID("localhost:8091", "foo")

	for j := 0; j < 10; j++ {
		go func() {
			for i := 0; i < 100000; i++ {
				remote.Tell(message)
			}
		}()
	}
	wg.Wait()
}
