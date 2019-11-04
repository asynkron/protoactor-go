package main

import (
	"runtime"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/remote"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// empty actor just to have something to remote spawn
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	remote.Register("remote", props)

	remote.Start("127.0.0.1:8080")

	console.ReadLine()
}
