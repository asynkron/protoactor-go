package main

import (
	"flag"
	fmt "fmt"
	"strings"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/automanaged"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/google/uuid"
)

func main() {

	remotingPort, clusteringPort, clusterMembers := getArgs()

	cluster := startNode(remotingPort, clusteringPort, clusterMembers)

	//publisherProps := actor.PropsFromProducer(func() actor.Actor { return NewPublisherActor(cluster) })
	cancelPublisher := publish(cluster)
	//subscriberProps := actor.PropsFromProducer(func() actor.Actor { return NewSubscriberActor() })
	cancelSubscriber := subscribe(cluster)

	//publisher := cluster.ActorSystem.Root.Spawn(publisherProps)
	//subscriber := cluster.ActorSystem.Root.Spawn(subscriberProps)

	console.ReadLine()

	//cluster.ActorSystem.Root.StopFuture(publisher).Wait()
	cancelPublisher()
	//cluster.ActorSystem.Root.StopFuture(subscriber).Wait()
	cancelSubscriber()

	cluster.Shutdown(true)
}

func startNode(remotingPort int, clusteringPort int, clusterMembers []string) *cluster.Cluster {
	system := actor.NewActorSystem()

	provider := automanaged.NewWithConfig(2*time.Second, clusteringPort, clusterMembers...)
	config := remote.Configure("localhost", remotingPort)

	clusterConfig := cluster.Configure("my-cluster", provider, config)
	cluster := cluster.New(system, clusterConfig)

	cluster.Start()

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
				cluster.MemberList.BroadcastEvent(event)
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
