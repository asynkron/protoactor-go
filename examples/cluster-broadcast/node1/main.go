package main

import (
	"fmt"
	"github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/cluster/consul"
	"github.com/otherview/protoactor-go/examples/cluster-broadcast/shared"
	"github.com/otherview/protoactor-go/remote"
	"log"
	"time"
)

func main () {
	startNode(8080)

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()
	calcAdd("Eggs", 1)
	calcAdd("Eggs", 10)

	calcAdd("Bananas", 1000)

	calcAdd("Meat", 3)
	calcAdd("Meat", 9000)

	getAll()

	console.ReadLine()

	cluster.Shutdown(true)
}

func startNode(port int64)  {
	// how long before the grain poisons itself
	timeout := 10 * time.Minute

	// this node knows about Hello kind
	remote.Register("Calculator", actor.PropsFromProducer(func() actor.Actor {
		return &shared.CalculatorActor{
			Timeout: &timeout,
		}
	}))

	// this node knows about Hello kind
	remote.Register("Tracker", actor.PropsFromProducer(func() actor.Actor {
		return &shared.TrackerActor{
			Timeout: &timeout,
		}
	}))

	shared.CalculatorFactory(func() shared.Calculator {
		return &shared.CalcGrain{}
	})

	shared.TrackerFactory(func() shared.Tracker {
		return &shared.TrackGrain{}
	})

	cp, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	cluster.Start("mycluster", fmt.Sprintf("127.0.0.1:%v", port), cp)
}

func calcAdd(grainId string, addNumber int64)  {
	calcGrain := shared.GetCalculatorGrain(grainId)
	total1, err := calcGrain.Add(&shared.NumberRequest{ Number: addNumber})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Grain: %v - Total: %v \n", calcGrain.ID, total1.Number)
}

func getAll()  {
	trackerGrain := shared.GetTrackerGrain("singleTrackerGrain")
	totals, err := trackerGrain.BroadcastGetCounts(&shared.Noop{})
	if err != nil {
		panic(err)
	}

	fmt.Println("--- Totals ---")
	for grainId, count := range totals.Totals {
		fmt.Printf("%v : %v\n", grainId, count)
	}
}