package main

import (
	"fmt"
	"time"

	"github.com/AsynkronIT/gam/examples/remoteactivate/messages"
	"github.com/AsynkronIT/gam/remoting"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	timeout := 5 * time.Second
	remoting.Start("127.0.0.1:8081")
	pid, _ := remoting.RemoteActivate("127.0.0.1:8080", "remote", "hello", timeout)
	res, _ := pid.RequestFuture(&messages.HelloRequest{}, timeout).Result()
	response := res.(*messages.HelloResponse)
	fmt.Printf("Response from remote %v", response.Message)

	console.ReadLine()
}
