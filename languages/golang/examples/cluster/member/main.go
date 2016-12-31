package main

import (
	"fmt"
	"log"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor/languages/golang/examples/cluster/shared"
	"github.com/AsynkronIT/protoactor/languages/golang/src/cluster"
	"github.com/AsynkronIT/protoactor/languages/golang/src/cluster/grain"
)

const (
	timeout = 1 * time.Second
)

func main() {
	cluster.Start("127.0.0.1:0", "127.0.0.1:7711")
	sync()
	async()

	console.ReadLine()
}

func sync() {
	hello := shared.GetHelloGrain("abc")
	options := []grain.GrainCallOption{grain.WithTimeout(5 * time.Second), grain.WithRetry(5)}
	res, err := hello.SayHello(&shared.HelloRequest{Name: "GAM"}, options...)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Message from SayHello: %v", res.Message)
	log.Println("Starting")
	for i := 0; i < 10000; i++ {
		x := shared.GetHelloGrain(fmt.Sprintf("hello%v", i))
		x.SayHello(&shared.HelloRequest{Name: "GAM"})
	}
	log.Println("Done")
}

func async() {
	hello := shared.GetHelloGrain("abc")
	c, e := hello.AddChan(&shared.AddRequest{A: 123, B: 456})

	for {
		select {
		case <-time.After(100 * time.Millisecond):
			log.Println("Tick..") //this might not happen if res returns fast enough
		case err := <-e:
			log.Fatal(err)
		case res := <-c:
			log.Printf("Result is %v", res.Result)
			return
		}
	}
}
