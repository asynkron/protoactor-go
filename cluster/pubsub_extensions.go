package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
)

// Publisher creates a new PubSub publisher that publishes messages directly to the TopicActor
func (c *Cluster) Publisher() Publisher {
	return NewPublisher(c)
}

// BatchingProducer create a new PubSub batching producer for specified topic, that publishes directly to the topic actor
func (c *Cluster) BatchingProducer(topic string, opts ...BatchProducerConfigOption) *BatchingProducer {
	return NewBatchingProducer(c.Publisher(), topic, opts...)
}

// SubscribeByPid subscribes to a PubSub topic by subscriber PID
func (c *Cluster) SubscribeByPid(topic string, pid *actor.PID, opts ...GrainCallOption) error {
	_, err := c.Call(topic, TopicActorKind, &SubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_Pid{Pid: pid}},
	}, opts...)
	return err
}

// SubscribeByClusterIdentity subscribes to a PubSub topic by cluster identity
func (c *Cluster) SubscribeByClusterIdentity(topic string, identity ClusterIdentity, opts ...GrainCallOption) error {
	_, err := c.Call(topic, TopicActorKind, &SubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: &identity}},
	}, opts...)
	return err
}

// SubscribeWithReceive subscribe to a PubSub topic by providing a Receive function, that will be used to spawn a subscriber actor
func (c *Cluster) SubscribeWithReceive(topic string, receive actor.ReceiveFunc, opts ...GrainCallOption) error {
	props := actor.PropsFromFunc(receive)
	pid := c.ActorSystem.Root.Spawn(props)
	return c.SubscribeByPid(topic, pid, opts...)
}

// UnsubscribeByPid unsubscribes from a PubSub topic by subscriber PID
func (c *Cluster) UnsubscribeByPid(topic string, pid *actor.PID, opts ...GrainCallOption) error {
	_, err := c.Call(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_Pid{Pid: pid}},
	}, opts...)
	return err
}

// UnsubscribeByClusterIdentity unsubscribes from a PubSub topic by cluster identity
func (c *Cluster) UnsubscribeByClusterIdentity(topic string, identity ClusterIdentity, opts ...GrainCallOption) error {
	_, err := c.Call(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: &identity}},
	}, opts...)
	return err
}

// UnsubscribeByIdentityAndKind unsubscribes from a PubSub topic by cluster identity
func (c *Cluster) UnsubscribeByIdentityAndKind(topic string, identity string, kind string, opts ...GrainCallOption) error {
	_, err := c.Call(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: &ClusterIdentity{Identity: identity, Kind: kind}}},
	}, opts...)
	return err
}
