package main

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/examples/cluster/shared"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	cluster.Start("127.0.0.1:0", "127.0.0.1:7711")
	timeout := 1 * time.Second

	hello := shared.GetHelloGrain("abc")
	res, err := hello.SayHello(&shared.HelloRequest{Name: "GAM"}, timeout)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Message from SayHello: %v", res.Message)

	res2, err := hello.Add(&shared.AddRequest{A: 123, B: 456}, timeout)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Result is %v", res2.Result)

	console.ReadLine()
}
