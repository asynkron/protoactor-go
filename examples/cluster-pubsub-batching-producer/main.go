package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	c := startNode()

	const topic = "my-topic"
	var deliveredCount int32 = 0
	for i := 0; i < 3; i++ {
		_, _ = c.SubscribeWithReceive(topic, func(context actor.Context) {
			switch context.Message().(type) {
			case *PingMessage:
				atomic.AddInt32(&deliveredCount, 1)
			}
		})
	}
	const count = 1_000_000
	fmt.Println("Starting producer...")

	producer := c.BatchingProducer(topic)

	start := time.Now()

	tasks := make([]*cluster.ProduceProcessInfo, count)
	for i := 0; i < count; i++ {
		info, err := producer.Produce(context.Background(), &PingMessage{Data: int32(i)})
		if err != nil {
			panic(err)
		}
		tasks[i] = info
	}
	for _, task := range tasks {
		<-task.Finished
	}
	elapsed := time.Since(start)
	producer.Dispose()
	c.Shutdown(true)

	fmt.Printf("Sent: %d, delivered: %d, msg/s %f\n", count, deliveredCount, float64(deliveredCount)/elapsed.Seconds())

	console.ReadLine()
}

func startNode() *cluster.Cluster {
	// how long before the grain poisons itself
	system := actor.NewActorSystem()

	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)

	cluster := cluster.New(system, clusterConfig)

	cluster.StartMember()

	return cluster
}
