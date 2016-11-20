package main

import (
	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting"
	console "github.com/AsynkronIT/goconsole"

	"runtime"

	"github.com/AsynkronIT/gam/examples/remotebenchmark/messages"
)

// import "runtime/pprof"

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	messageCount := 1000000

	remoting.Start("127.0.0.1:8081")

	remote := actor.NewPID("127.0.0.1:8080", "remote")

	message := &messages.Ping{}
	for i := 0; i < messageCount; i++ {
		remote.Tell(message)
	}
	console.ReadLine()
}
