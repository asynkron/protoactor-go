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
	c := hello.SayHelloChan(&shared.HelloRequest{Name: "GAM"}, timeout)
	res := <-c //channel 
	if res.Err != nil {
		log.Fatal(res.Err)
	} else {
		log.Printf("Message from grain %v", res.Value.Message)
	}

	res2, err := hello.Add(&shared.AddRequest{A: 123, B: 456}, timeout)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Result is %v", res2.Result)

	console.ReadLine()
}
