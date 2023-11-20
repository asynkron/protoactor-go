package cluster_test_tool

import (
	"errors"
	"log/slog"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

const (
	PubSubSubscriberKind        = "Subscriber"
	PubSubTimeoutSubscriberKind = "TimeoutSubscriber"
)

type PubSubClusterFixture struct {
	*BaseClusterFixture

	useDefaultTopicRegistration bool
	t                           testing.TB

	Deliveries     []Delivery
	DeliveriesLock *sync.RWMutex

	subscriberStore cluster.KeyValueStore[*cluster.Subscribers]
}

func NewPubSubClusterFixture(t testing.TB, clusterSize int, useDefaultTopicRegistration bool, opts ...ClusterFixtureOption) *PubSubClusterFixture {
	lock := &sync.RWMutex{}
	store := NewInMemorySubscriberStore()
	fixture := &PubSubClusterFixture{
		t:                           t,
		useDefaultTopicRegistration: useDefaultTopicRegistration,
		Deliveries:                  []Delivery{},
		DeliveriesLock:              lock,
		subscriberStore:             store,
	}

	pubSubOpts := []ClusterFixtureOption{
		WithGetClusterKinds(func() []*cluster.Kind {
			kinds := []*cluster.Kind{
				cluster.NewKind(PubSubSubscriberKind, fixture.subscriberProps()),
				cluster.NewKind(PubSubTimeoutSubscriberKind, fixture.timeoutSubscriberProps()),
			}
			if !fixture.useDefaultTopicRegistration {
				kinds = append(kinds, cluster.NewKind(cluster.TopicActorKind, actor.PropsFromProducer(func() actor.Actor {
					return cluster.NewTopicActor(store, slog.Default())
				})))
			}
			return kinds
		}),
		WithClusterConfigure(func(config *cluster.Config) *cluster.Config {
			cluster.WithRequestTimeout(time.Second * 1)(config)
			cluster.WithPubSubSubscriberTimeout(time.Second * 2)(config)
			return config
		}),
	}
	pubSubOpts = append(pubSubOpts, opts...)

	fixture.BaseClusterFixture = NewBaseInMemoryClusterFixture(clusterSize, pubSubOpts...)
	return fixture
}

func (p *PubSubClusterFixture) RandomMember() *cluster.Cluster {
	members := p.BaseClusterFixture.GetMembers()
	return members[rand.Intn(len(members))]
}

// VerifyAllSubscribersGotAllTheData verifies that all subscribers got all the data
func (p *PubSubClusterFixture) VerifyAllSubscribersGotAllTheData(subscriberIds []string, numMessages int) {
	WaitUntil(p.t, func() bool {
		p.DeliveriesLock.RLock()
		defer p.DeliveriesLock.RUnlock()
		return len(p.Deliveries) == numMessages*len(subscriberIds)
	}, "All messages should be delivered ", DefaultWaitTimeout*1000)

	p.DeliveriesLock.RLock()
	defer p.DeliveriesLock.RUnlock()

	expected := make([]Delivery, 0, len(subscriberIds))
	for _, subscriberId := range subscriberIds {
		for i := 0; i < numMessages; i++ {
			expected = append(expected, Delivery{
				Identity: subscriberId,
				Data:     i,
			})
		}
	}
	assert.ElementsMatch(p.t, expected, p.Deliveries)
}

// SubscribeAllTo subscribes all the given subscribers to the given topic
func (p *PubSubClusterFixture) SubscribeAllTo(topic string, subscriberIds []string) {
	for _, subscriberId := range subscriberIds {
		p.SubscribeTo(topic, subscriberId, PubSubSubscriberKind)
	}
}

// UnSubscribeAllFrom unsubscribes all the given subscribers from the given topic
func (p *PubSubClusterFixture) UnSubscribeAllFrom(topic string, subscriberIds []string) {
	for _, subscriberId := range subscriberIds {
		p.UnSubscribeTo(topic, subscriberId, PubSubSubscriberKind)
	}
}

// SubscribeTo subscribes the given subscriber to the given topic
func (p *PubSubClusterFixture) SubscribeTo(topic, identity, kind string) {
	c := p.RandomMember()
	res, err := c.SubscribeByClusterIdentity(topic, cluster.NewClusterIdentity(identity, kind), cluster.WithTimeout(time.Second*5))
	assert.NoError(p.t, err, kind+"/"+identity+" should be able to subscribe to topic "+topic)
	assert.NotNil(p.t, res, kind+"/"+identity+" subscribing should not time out on topic "+topic)
}

// UnSubscribeTo unsubscribes the given subscriber from the given topic
func (p *PubSubClusterFixture) UnSubscribeTo(topic, identity, kind string) {
	c := p.RandomMember()
	res, err := c.UnsubscribeByClusterIdentity(topic, cluster.NewClusterIdentity(identity, kind), cluster.WithTimeout(time.Second*5))
	assert.NoError(p.t, err, kind+"/"+identity+" should be able to unsubscribe from topic "+topic)
	assert.NotNil(p.t, res, kind+"/"+identity+" subscribing should not time out on topic "+topic)
}

// PublishData publishes the given message to the given topic
func (p *PubSubClusterFixture) PublishData(topic string, data int) (*cluster.PublishResponse, error) {
	c := p.RandomMember()
	return c.Publisher().Publish(context.Background(), topic, &DataPublished{Data: int32(data)}, cluster.WithTimeout(time.Second*5))
}

// PublishDataBatch publishes the given messages to the given topic
func (p *PubSubClusterFixture) PublishDataBatch(topic string, data []int) (*cluster.PublishResponse, error) {
	batches := make([]interface{}, 0)
	for _, d := range data {
		batches = append(batches, &DataPublished{Data: int32(d)})
	}

	c := p.RandomMember()
	return c.Publisher().PublishBatch(context.Background(), topic, &cluster.PubSubBatch{Envelopes: batches}, cluster.WithTimeout(time.Second*5))
}

// SubscriberIds returns the subscriber ids
func (p *PubSubClusterFixture) SubscriberIds(prefix string, count int) []string {
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		ids = append(ids, prefix+strconv.Itoa(i))
	}
	return ids
}

// GetSubscribersForTopic returns the subscribers for the given topic
func (p *PubSubClusterFixture) GetSubscribersForTopic(topic string) (*cluster.Subscribers, error) {
	return p.subscriberStore.Get(context.Background(), topic)
}

// ClearDeliveries clears the deliveries
func (p *PubSubClusterFixture) ClearDeliveries() {
	p.DeliveriesLock.Lock()
	defer p.DeliveriesLock.Unlock()
	p.Deliveries = make([]Delivery, 0)
}

// subscriberProps returns the props for the subscriber actor
func (p *PubSubClusterFixture) subscriberProps() *actor.Props {
	return actor.PropsFromFunc(func(context actor.Context) {
		if msg, ok := context.Message().(*DataPublished); ok {
			identity := cluster.GetClusterIdentity(context)

			p.AppendDelivery(Delivery{
				Identity: identity.Identity,
				Data:     int(msg.Data),
			})
			context.Respond(&Response{})
		}
	})
}

// timeoutSubscriberProps returns the props for the subscriber actor
func (p *PubSubClusterFixture) timeoutSubscriberProps() *actor.Props {
	return actor.PropsFromFunc(func(context actor.Context) {
		if msg, ok := context.Message().(*DataPublished); ok {
			time.Sleep(time.Second * 4) // 4 seconds is longer than the configured subscriber timeout

			identity := cluster.GetClusterIdentity(context)
			p.AppendDelivery(Delivery{
				Identity: identity.Identity,
				Data:     int(msg.Data),
			})
			context.Respond(&Response{})
		}
	})
}

// AppendDelivery appends a delivery to the deliveries slice
func (p *PubSubClusterFixture) AppendDelivery(delivery Delivery) {
	p.DeliveriesLock.Lock()
	p.Deliveries = append(p.Deliveries, delivery)
	p.DeliveriesLock.Unlock()
}

type Delivery struct {
	Identity string
	Data     int
}

func NewInMemorySubscriberStore() *InMemorySubscribersStore[*cluster.Subscribers] {
	return &InMemorySubscribersStore[*cluster.Subscribers]{
		store: &sync.Map{},
	}
}

type InMemorySubscribersStore[T any] struct {
	store *sync.Map // map[string]T
}

func (i *InMemorySubscribersStore[T]) Set(_ context.Context, key string, value T) error {
	i.store.Store(key, value)
	return nil
}

func (i *InMemorySubscribersStore[T]) Get(_ context.Context, key string) (T, error) {
	var r T
	value, ok := i.store.Load(key)
	if !ok {
		return r, errors.New("not found")
	}
	return value.(T), nil
}

func (i *InMemorySubscribersStore[T]) Clear(_ context.Context, key string) error {
	i.store.Delete(key)
	return nil
}
