package main

import "github.com/rogeralsing/gam/actor"
import "github.com/rogeralsing/gam/remoting"
import "fmt"
import "log"
import "bufio"
import "os"
//import "time"
//import "runtime/pprof"
import "github.com/rogeralsing/gam/examples/remoting/messages"

type MyActor struct{
	count int
}

func (state *MyActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case string:
		fmt.Println(msg)
	case *messages.Response:
		state.count++
		if state.count % 10000 == 0 {
			log.Println(state.count)
		}
	}
}

func main() {
	
	// f, err := os.Create("cpuprofile")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()
	
	
	remoting.StartServer("localhost:8090")

	pid := actor.SpawnTemplate(&MyActor{})
	message := &messages.Echo{Message: "hej", Sender: pid}
	remote := actor.NewPID("localhost:8091", "foo")
	for i := 0; i < 1000000; i++ {
		remote.Tell(message)
	}

	bufio.NewReader(os.Stdin).ReadString('\n')
	//time.Sleep(15 * time.Second)
}
