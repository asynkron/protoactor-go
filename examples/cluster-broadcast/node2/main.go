package main

import (
	"fmt"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/cluster/automanaged"
	"github.com/otherview/protoactor-go/examples/cluster-broadcast/shared"
	"github.com/otherview/protoactor-go/remote"
)

func main() {
	startNode(8081)

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()

	fmt.Print("\nAdding 1 Egg - Enter\n")
	console.ReadLine()
	calcAdd("Eggs", 1)

	fmt.Print("\nAdding 10 Egg - Enter\n")
	console.ReadLine()
	calcAdd("Eggs", 10)

	fmt.Print("\nAdding 100 Bananas - Enter\n")
	console.ReadLine()
	calcAdd("Bananas", 100)

	fmt.Print("\nAdding 2 Meat - Enter\n")
	console.ReadLine()
	calcAdd("Meat", 3)

	getAll()

	console.ReadLine()

	cluster.Shutdown(true)
}

func startNode(port int64) {
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

	clusterProvider := automanaged.NewWithConfig(2*time.Second, 6331, "localhost:6330", "localhost:6331")
	cluster.Start("mycluster", fmt.Sprintf("localhost:%v", port), clusterProvider)
}

func calcAdd(grainId string, addNumber int64) {
	calcGrain := shared.GetCalculatorGrain(grainId)
	total1, err := calcGrain.Add(&shared.NumberRequest{Number: addNumber})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Grain: %v - Total: %v \n", calcGrain.ID, total1.Number)
}

func getAll() {
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
