package cluster

import (
	"context"
	"google.golang.org/protobuf/types/known/durationpb"
	"time"
)

type PublisherConfig struct {
	IdleTimeout time.Duration
}

type Publisher interface {
	// Initialize the internal mechanisms of this publisher.
	Initialize(ctx context.Context, topic string, config PublisherConfig) (*Acknowledge, error)

	// PublishBatch publishes a batch of messages to the topic.
	PublishBatch(ctx context.Context, topic string, batch *PubSubBatch) (*PublishResponse, error)
}

type defaultPublisher struct {
	cluster *Cluster
}

func NewPublisher(cluster *Cluster) Publisher {
	return &defaultPublisher{
		cluster: cluster,
	}
}

func (p *defaultPublisher) Initialize(ctx context.Context, topic string, config PublisherConfig) (*Acknowledge, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		res, err := p.cluster.Call(topic, TopicActorKind, &Initialize{
			IdleTimeout: durationpb.New(config.IdleTimeout),
		})
		return res.(*Acknowledge), err
	}
}

func (p *defaultPublisher) PublishBatch(ctx context.Context, topic string, batch *PubSubBatch) (*PublishResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		res, err := p.cluster.Call(topic, TopicActorKind, batch)
		return res.(*PublishResponse), err
	}
}
