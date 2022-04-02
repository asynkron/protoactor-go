package main

import (
	"cluster-broadcast/shared"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	automanaged "github.com/asynkron/protoactor-go/cluster/clusterproviders/_automanaged"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/partition"
	"github.com/asynkron/protoactor-go/remote"
	"time"
)

func main() {
	c := startNode(8080)

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()

	fmt.Print("\nAdding 1 Egg - Enter\n")
	console.ReadLine()
	calcAdd(c, "Eggs", 1)

	fmt.Print("\nAdding 10 Egg - Enter\n")
	console.ReadLine()
	calcAdd(c, "Eggs", 10)

	fmt.Print("\nAdding 100 Bananas - Enter\n")
	console.ReadLine()
	calcAdd(c, "Bananas", 100)

	fmt.Print("\nAdding 2 Meat - Enter\n")
	console.ReadLine()
	calcAdd(c, "Meat", 3)
	calcAdd(c, "Meat", 9000)

	getAll(c)

	console.ReadLine()

	c.Shutdown(true)
}

func startNode(port int64) *cluster.Cluster {
	// how long before the grain poisons itself
	timeout := 10 * time.Minute

	system := actor.NewActorSystem()

	calcKind := cluster.NewKind("Calculator", actor.PropsFromProducer(func() actor.Actor {
		return &shared.CalculatorActor{
			Timeout: timeout,
		}
	}))
	trackerKind := cluster.NewKind("Tracker", actor.PropsFromProducer(func() actor.Actor {
		return &shared.TrackerActor{
			Timeout: timeout,
		}
	}))

	provider := automanaged.NewWithConfig(2*time.Second, 6331, "localhost:6330", "localhost:6331")
	lookup := partition.New()
	config := remote.Configure("localhost", 0)

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config, calcKind, trackerKind)
	cluster := cluster.New(system, clusterConfig)

	shared.CalculatorFactory(func() shared.Calculator {
		return &shared.CalcGrain{}
	})

	shared.TrackerFactory(func() shared.Tracker {
		return &shared.TrackGrain{}
	})

	cluster.StartMember()

	return cluster
}

func calcAdd(cluster *cluster.Cluster, grainId string, addNumber int64) {
	calcGrain := shared.GetCalculatorGrainClient(cluster, grainId)
	total1, err := calcGrain.Add(&shared.NumberRequest{Number: addNumber})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Grain: %v - Total: %v \n", calcGrain.Identity, total1.Number)
}

func getAll(cluster *cluster.Cluster) {
	trackerGrain := shared.GetTrackerGrainClient(cluster, "singleTrackerGrain")
	totals, err := trackerGrain.BroadcastGetCounts(&shared.Noop{})
	if err != nil {
		panic(err)
	}

	fmt.Println("--- Totals ---")
	for grainId, count := range totals.Totals {
		fmt.Printf("%v : %v\n", grainId, count)
	}
}
