package main

import (
	"runtime"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	system  = actor.NewActorSystem()
	context = system.Root
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	cfg := remote.Configure("127.0.0.1", 8080, remote.WithKinds(remote.NewKind("remote", props)))

	r := remote.NewRemote(system, cfg)
	r.Register("remote", props)
	r.Start()

	// empty actor just to have something to remote spawn

	console.ReadLine()
}
