package cluster

import (
	"fmt"

	"github.com/awevoke/protoactor-go/actor"
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
	resp, ok := res.(*SubscribeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T, expected *SubscribeResponse", res)
	}
	return resp, nil
}

// SubscribeByClusterIdentity subscribes to a PubSub topic by cluster identity
func (c *Cluster) SubscribeByClusterIdentity(topic string, identity *ClusterIdentity, opts ...GrainCallOption) (*SubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &SubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: identity}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	resp, ok := res.(*SubscribeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T, expected *SubscribeResponse", res)
	}
	return resp, nil
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
	resp, ok := res.(*UnsubscribeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T, expected *UnsubscribeResponse", res)
	}
	return resp, nil
}

// UnsubscribeByClusterIdentity unsubscribes from a PubSub topic by cluster identity
func (c *Cluster) UnsubscribeByClusterIdentity(topic string, identity *ClusterIdentity, opts ...GrainCallOption) (*UnsubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: identity}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	resp, ok := res.(*UnsubscribeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T, expected *UnsubscribeResponse", res)
	}
	return resp, nil
}

// UnsubscribeByIdentityAndKind unsubscribes from a PubSub topic by cluster identity
func (c *Cluster) UnsubscribeByIdentityAndKind(topic string, identity string, kind string, opts ...GrainCallOption) (*UnsubscribeResponse, error) {
	res, err := c.Request(topic, TopicActorKind, &UnsubscribeRequest{
		Subscriber: &SubscriberIdentity{Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: NewClusterIdentity(identity, kind)}},
	}, opts...)
	if err != nil {
		return nil, err
	}
	resp, ok := res.(*UnsubscribeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T, expected *UnsubscribeResponse", res)
	}
	return resp, nil
}
