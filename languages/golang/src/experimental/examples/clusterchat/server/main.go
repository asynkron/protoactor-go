package main

import (
	"runtime"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/experimental/examples/clusterchat/messages"
	"github.com/AsynkronIT/goconsole"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cluster.Start("127.0.0.1:7711")
	_ = messages.GetChatServerGrain("server")
	console.ReadLine()
}
