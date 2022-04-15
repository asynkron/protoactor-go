package main

import (
	"fmt"
	"time"

	"cluster-broadcast/shared"

	"github.com/asynkron/protoactor-go/cluster/clusterproviders/automanaged"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	c := startNode(8081)

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
	system := actor.NewActorSystem()

	provider := automanaged.NewWithConfig(2*time.Second, 6330, "localhost:6330", "localhost:6331")
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	calculatorKind := shared.NewCalculatorKind(func() shared.Calculator {
		return &shared.CalcGrain{}
	}, 0)

	trackerKind := shared.NewTrackerKind(func() shared.Tracker {
		return &shared.TrackGrain{}
	}, 0)

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config,
		cluster.WithKinds(calculatorKind, trackerKind))

	cluster := cluster.New(system, clusterConfig)

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
