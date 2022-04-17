package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"remotebenchmark/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type localActor struct {
	count        int
	wgStop       *sync.WaitGroup
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
	}
}

func newLocalActor(stop *sync.WaitGroup, messageCount int) actor.Producer {
	return func() actor.Actor {
		return &localActor{
			wgStop:       stop,
			messageCount: messageCount,
		}
	}
}

var (
	cpuprofile   = flag.String("cpuprofile", "", "write cpu profile to file")
	blockProfile = flag.String("blockprof", "", "execute contention profiling and save results here")
)

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Check for lock contention profiling
	if *blockProfile != "" {
		prof, err := os.Create(*blockProfile)
		if err != nil {
			log.Fatal(err)
		}
		runtime.SetBlockProfileRate(1)
		defer func() {
			pprof.Lookup("block").WriteTo(prof, 0)
		}()
	}

	// runtime.GOMAXPROCS(runtime.NumCPU() * 1)
	// runtime.GC()

	messageCount := 1000000
	// remote.DefaultSerializerID = 1
	system := actor.NewActorSystem()
	r := remote.NewRemote(system, remote.Configure("127.0.0.1", 8081 /*, remote.WithCallOptions(grpc.UseCompressor(gzip.Name))*/))
	r.Start()

	rootContext := system.Root

	run := true
	go func() {
		for run == true {
			var wg sync.WaitGroup
			props := actor.
				PropsFromProducer(newLocalActor(&wg, messageCount),
					actor.WithMailbox(actor.Bounded(1000000)))

			pid := rootContext.Spawn(props)

			pidResponse, err := r.Spawn("127.0.0.1:12000", "echo", time.Second*2000)
			if err != nil || pidResponse.StatusCode != 0 {
				rootContext.Stop(pid)
				return
			}
			remotePid := pidResponse.Pid
			msg := messages.StartRemote{Sender: pid}
			rootContext.RequestFuture(remotePid, &msg, 5*time.Second).Wait()
			wg.Add(1)

			start := time.Now()
			log.Println("Starting to send")

			message := &messages.Ping{}
			for i := 0; i < messageCount; i++ {
				rootContext.Send(remotePid, message)
			}

			wg.Wait()
			elapsed := time.Since(start)
			log.Printf("Elapsed %s", elapsed)

			x := int(float32(messageCount*2) / (float32(elapsed) / float32(time.Second)))
			log.Printf("Msg per sec %v", x)
			rootContext.Stop(remotePid)
			rootContext.Stop(pid)
		}
	}()
	console.ReadLine()
	run = false
	console.ReadLine()
	r.Shutdown(true)
}
