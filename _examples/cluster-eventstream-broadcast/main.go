package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/cluster/clusterproviders/automanaged"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/google/uuid"
)

func main() {
	remotingPort, clusteringPort, clusterMembers := getArgs()

	cluster := startNode(remotingPort, clusteringPort, clusterMembers)

	cancelPublisher := publish(cluster)
	cancelSubscriber := subscribe(cluster)

	console.ReadLine()

	cancelPublisher()
	cancelSubscriber()

	cluster.Shutdown(true)
}

func startNode(remotingPort int, clusteringPort int, clusterMembers []string) *cluster.Cluster {
	system := actor.NewActorSystem()

	provider := automanaged.NewWithConfig(2*time.Second, clusteringPort, clusterMembers...)
	lookup := disthash.New()
	config := remote.Configure("localhost", remotingPort)

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	cluster := cluster.New(system, clusterConfig)

	cluster.StartMember()

	return cluster
}

func publish(cluster *cluster.Cluster) (cancel func()) {
	id := uuid.New().String()[:8]
	done := make(chan struct{})

	ticker := time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Printf("==>> Publisher %s sending message\n", id)
				event := &MyEvent{
					Description: fmt.Sprintf("Hello from %s at %s", id, time.Now().Format(time.RFC3339)),
				}
				cluster.MemberList.BroadcastEvent(event, true)
			}
		}
	}()

	return func() {
		ticker.Stop()
		done <- struct{}{}
	}
}

func subscribe(cluster *cluster.Cluster) (cancel func()) {
	subscription := cluster.ActorSystem.EventStream.Subscribe(func(evt interface{}) {
		if event, ok := evt.(*MyEvent); ok {
			fmt.Printf("<<== Subscriber received event: %s\n", event.Description)
		}
	})

	return func() {
		cluster.ActorSystem.EventStream.Unsubscribe(subscription)
	}
}

func getArgs() (remotingPort int, clusteringPort int, clusterMembers []string) {
	flag.IntVar(&remotingPort, "remoting-port", 18080, "port for actor remote communication")
	flag.IntVar(&clusteringPort, "clustering-port", 28080, "port for cluster provider communication")

	var membersString string
	flag.StringVar(&membersString, "members", "localhost:28080", "cluster members e.g. `--members=localhost:28080,localhost:28081")

	flag.Parse()

	clusterMembers = strings.Split(membersString, ",")

	return
}
