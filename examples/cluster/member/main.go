package main

import (
	"fmt"
	"time"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/examples/cluster/shared"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	cluster.Start("127.0.0.1:0", "127.0.0.1:7711")
	fmt.Println("Running")
	time.Sleep(10 * time.Second)
	pid := cluster.Get("myfirst", shared.Type1)
	pid.Tell("hello")
	console.ReadLine()
}
