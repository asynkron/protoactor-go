package main

import (
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/examples/remotelatency/messages"
	"github.com/AsynkronIT/gam/remoting"
	console "github.com/AsynkronIT/goconsole"

	"runtime"
)

// import "runtime/pprof"

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	messageCount := 1000000

	remoting.Start("127.0.0.1:8081", remoting.WithBatchSize(10000))

	remote := actor.NewPID("127.0.0.1:8080", "remote")
	res, _ := remote.AskFuture(&messages.Start{}, 5*time.Second)
	res.Wait()

	for i := 0; i < messageCount; i++ {
		message := &messages.Ping{
			Time: makeTimestamp(),
		}
		remote.Tell(message)
		if i%1000 == 0 {
			time.Sleep(500)
		}
	}
	console.ReadLine()
}
