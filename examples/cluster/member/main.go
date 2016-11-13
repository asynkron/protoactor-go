package main

import (
	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/examples/cluster/shared"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	cluster.Start("127.0.0.1:0", "127.0.0.1:7711")

	pid := cluster.Get("myfirst", shared.Type1)
	pid.Tell(&shared.HelloMessage{})
	console.ReadLine()
}
