package main

import (
	"fmt"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/examples/remoteactivate/messages"
	"github.com/otherview/protoactor-go/remote"
)

func main() {
	timeout := 5 * time.Second
	remote.Start("127.0.0.1:8081")
	pidResp, _ := remote.SpawnNamed("127.0.0.1:8080", "remote", "hello", timeout)
	pid := pidResp.Pid
	res, _ := actor.EmptyRootContext.RequestFuture(pid, &messages.HelloRequest{}, timeout).Result()
	response := res.(*messages.HelloResponse)
	fmt.Printf("Response from remote %v", response.Message)

	console.ReadLine()
}
