package cluster

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

type PublisherConfig struct {
	IdleTimeout time.Duration
}

type Publisher interface {
	// Initialize the internal mechanisms of this publisher.
	Initialize(ctx context.Context, topic string, config PublisherConfig) (*Acknowledge, error)

	// PublishBatch publishes a batch of messages to the topic.
	PublishBatch(ctx context.Context, topic string, batch *PubSubBatch, opts ...GrainCallOption) (*PublishResponse, error)

	// Publish publishes a single message to the topic.
	Publish(ctx context.Context, topic string, message interface{}, opts ...GrainCallOption) (*PublishResponse, error)

	Logger() *slog.Logger
}

type defaultPublisher struct {
	cluster *Cluster
}

func NewPublisher(cluster *Cluster) Publisher {
	return &defaultPublisher{
		cluster: cluster,
	}
}

func (p *defaultPublisher) Logger() *slog.Logger {
	return p.cluster.Logger()
}

func (p *defaultPublisher) Initialize(ctx context.Context, topic string, config PublisherConfig) (*Acknowledge, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		res, err := p.cluster.Request(topic, TopicActorKind, &Initialize{
			IdleTimeout: durationpb.New(config.IdleTimeout),
		})
		if err != nil {
			return nil, err
		}
		return res.(*Acknowledge), err
	}
}

func (p *defaultPublisher) PublishBatch(ctx context.Context, topic string, batch *PubSubBatch, opts ...GrainCallOption) (*PublishResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		res, err := p.cluster.Request(topic, TopicActorKind, batch, opts...)
		if err != nil {
			return nil, err
		}
		return res.(*PublishResponse), err
	}
}

func (p *defaultPublisher) Publish(ctx context.Context, topic string, message interface{}, opts ...GrainCallOption) (*PublishResponse, error) {
	return p.PublishBatch(ctx, topic, &PubSubBatch{
		Envelopes: []interface{}{message},
	}, opts...)
}
