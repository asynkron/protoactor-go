package main

import (
	"log"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/examples/cluster/shared"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	cluster.Start("127.0.0.1:0", "127.0.0.1:7711")
	hello := shared.GetHelloGrain("abc")
	res := hello.SayHello(&shared.HelloRequest{Name: "GAM"})
	log.Printf("Message from grain %v", res.Message)
	res2 := hello.Add(&shared.AddRequest{A: 123, B: 456})
	log.Printf("Result is %v", res2.Result)
	log.Println("start")
	for i := 0; i < 10000; i++ {
		hello.Add(&shared.AddRequest{A: 123, B: 456})
	}
	log.Println("done")
	console.ReadLine()
}
