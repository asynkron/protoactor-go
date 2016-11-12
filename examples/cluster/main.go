package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/cluster"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	cluster.Start("127.0.0.1:7711")
	fmt.Println("Running")
	console.ReadLine()
}
