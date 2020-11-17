package main

import (
	"time"

	"remotelatency/messages"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"

	"runtime"
)

// import "runtime/pprof"

var (
	system      = actor.NewActorSystem()
	rootContext = system.Root
)

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
func main() {
	cfg := remote.Configure("127.0.0.1", 8081)
	cfg = cfg.WithEndpointManagerBatchSize(10000)
	r := remote.NewRemote(system, cfg)
	r.Start()

	runtime.GOMAXPROCS(runtime.NumCPU())

	messageCount := 1000000

	remote := actor.NewPID("127.0.0.1:8080", "remote")
	rootContext.RequestFuture(remote, &messages.Start{}, 5*time.Second).
		Wait()

	for i := 0; i < messageCount; i++ {
		message := &messages.Ping{
			Time: makeTimestamp(),
		}
		rootContext.Send(remote, message)
		if i%1000 == 0 {
			time.Sleep(500)
		}
	}
	console.ReadLine()
}
