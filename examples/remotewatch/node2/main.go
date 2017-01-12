package main

import (
	"runtime"

	"github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remoting"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	//empty actor just to have something to remote spawn
	props := actor.FromFunc(func(ctx actor.Context) {})
	remoting.Register("remote", props)

	remoting.Start("127.0.0.1:8080")

	console.ReadLine()
}
