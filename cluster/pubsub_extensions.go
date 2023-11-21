package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
)

// Publisher creates a new PubSub publisher that publishes messages directly to the TopicActor
func (c *Cluster) Publisher() Publisher {
	return NewPublisher(c)
}

// BatchingProducer create a new PubSub batching producer for specified topic, that publishes directly to the topic actor
func (c *Cluster) BatchingProducer(topic string, opts ...BatchingProducerConfigOption) *BatchingProducer {
	return NewBatchingProducer(c.Publisher(), topic, opts...)
}

// SubscribeByPid subscribes to a PubSub topic by subscriber PID
func (c *Cluster) SubscribeByPid(topic string, pid *actor.PID, opts ...GrainCallOption) (*SubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &SubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_Pid{Pid: pid}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	return res.(*SubscribeResponse), err
}

// SubscribeByClusterIdentity subscribes to a PubSub topic by cluster identity
func (c *Cluster) SubscribeByClusterIdentity(topic string, identity *ClusterIdentity, opts ...GrainCallOption) (*SubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &SubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: identity}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	return res.(*SubscribeResponse), err
}

// SubscribeWithReceive subscribe to a PubSub topic by providing a Receive function, that will be used to spawn a subscriber actor
func (c *Cluster) SubscribeWithReceive(topic string, receive actor.ReceiveFunc, opts ...GrainCallOption) (*SubscribeResponse, error) {
	props := actor.PropsFromFunc(receive)
	pid := c.ActorSystem.Root.Spawn(props)
	return c.SubscribeByPid(topic, pid, opts...)
}

// UnsubscribeByPid unsubscribes from a PubSub topic by subscriber PID
func (c *Cluster) UnsubscribeByPid(topic string, pid *actor.PID, opts ...GrainCallOption) (*UnsubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_Pid{Pid: pid}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	return res.(*UnsubscribeResponse), err
}

// UnsubscribeByClusterIdentity unsubscribes from a PubSub topic by cluster identity
func (c *Cluster) UnsubscribeByClusterIdentity(topic string, identity *ClusterIdentity, opts ...GrainCallOption) (*UnsubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: identity}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	return res.(*UnsubscribeResponse), err
}

// UnsubscribeByIdentityAndKind unsubscribes from a PubSub topic by cluster identity
func (c *Cluster) UnsubscribeByIdentityAndKind(topic string, identity string, kind string, opts ...GrainCallOption) (*UnsubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: NewClusterIdentity(identity, kind)}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	return res.(*UnsubscribeResponse), err
}
