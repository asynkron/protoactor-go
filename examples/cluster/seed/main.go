package main

import (
	"fmt"
	"log"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/examples/cluster/shared"
)

func main() {
	cluster.Start("127.0.0.1:7711")
	log.Println("starting")
	for i := 0; i < 100000; i++ {
		g := shared.GetHelloGrain(fmt.Sprintf("abc%v", i))
		g.SayHello(&shared.HelloRequest{Name: "Roger"})
	}
	log.Println("done")
	// hello :=

	// res, err := hello.SayHello(&shared.HelloRequest{Name: "Roger"})
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Printf("Message from grain %v", res.Message)
	// console.ReadLine()
}
