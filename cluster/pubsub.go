package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/extensions"
)

const PubSubDeliveryName = "$pubsub-delivery"

var pubsubExtensionID = extensions.NextExtensionID()

type PubSub struct {
	cluster *Cluster
}

func NewPubSub(cluster *Cluster) *PubSub {
	p := &PubSub{
		cluster: cluster,
	}
	cluster.ActorSystem.Extensions.Register(p)
	return p
}

// Start the PubSubMemberDeliveryActor
func (p *PubSub) Start() {
	props := actor.PropsFromProducer(func() actor.Actor {
		return NewPubSubMemberDeliveryActor(p.cluster.Config.PubSubConfig.SubscriberTimeout, p.cluster.Logger())
	})
	_, err := p.cluster.ActorSystem.Root.SpawnNamed(props, PubSubDeliveryName)
	if err != nil {
		panic(err) // let it crash
	}
	p.cluster.Logger().Info("Started Cluster PubSub")
}

func (p *PubSub) ExtensionID() extensions.ExtensionID {
	return pubsubExtensionID
}

type PubSubConfig struct {
	// SubscriberTimeout is a timeout used when delivering a message batch to a subscriber. Default is 5s.
	//
	// This value gets rounded to seconds for optimization of cancellation token creation. Note that internally,
	// cluster request is used to deliver messages to ClusterIdentity subscribers.
	SubscriberTimeout time.Duration
}

func newPubSubConfig() *PubSubConfig {
	return &PubSubConfig{
		SubscriberTimeout: 5 * time.Second,
	}
}

// GetPubSub returns the PubSub extension from the actor system
func GetPubSub(system *actor.ActorSystem) *PubSub {
	return system.Extensions.Get(pubsubExtensionID).(*PubSub)
}
