package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/examples/cluster/shared"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	cluster.Start("127.0.0.1:7711", "127.0.0.1:7711")
	fmt.Println("Running")
	pid := cluster.Get("myfirst", shared.Type1)
	pid.Tell(&shared.HelloMessage{})
	console.ReadLine()
}
