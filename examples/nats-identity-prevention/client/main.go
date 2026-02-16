package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"nats-identity-prevention/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

var concurrency = flag.Int("concurrency", 10, "Number of concurrent goroutines sending increments")

func main() {
	flag.Parse()

	// Set up Proto.Actor cluster as a client (no grain kinds)
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	// Client uses disthash — it doesn't need storage lookup
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	clusterConfig := cluster.Configure("identity-example", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatalf("Failed to start cluster client: %v", err)
	}
	defer c.Shutdown(true)

	// Wait briefly for cluster topology to settle
	fmt.Println("Client started, waiting for cluster topology...")
	time.Sleep(3 * time.Second)

	n := *concurrency
	fmt.Printf("Sending %d concurrent Increment requests to identity 'counter-1'\n", n)

	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			client := shared.GetCounterGrainClient(c, "counter-1")
			resp, err := client.Increment(&shared.IncrementRequest{Amount: 1})
			if err != nil {
				fmt.Printf("goroutine %d: error: %v\n", goroutineID, err)
				return
			}
			fmt.Printf("goroutine %d: count=%d\n", goroutineID, resp.Count)
		}(i)
	}

	wg.Wait()
	fmt.Println()

	// Verify final count
	fmt.Println("Verifying final count...")
	client := shared.GetCounterGrainClient(c, "counter-1")
	resp, err := client.GetCount(&shared.GetCountRequest{})
	if err != nil {
		fmt.Printf("Failed to get final count: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Final count: %d (expected: %d)\n", resp.Count, n)
	if int(resp.Count) == n {
		fmt.Println("SUCCESS: All increments were processed by a single activation.")
	} else {
		fmt.Println("WARNING: Count mismatch — possible duplicate activations or lost messages.")
	}

	fmt.Println("\nPress Ctrl+C to exit, or the client will exit automatically.")

	// Allow graceful exit via signal or automatic exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
	case <-time.After(5 * time.Second):
	}
}
