package cluster

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/eventstream"
	"golang.org/x/exp/maps"
)

const TopicActorKind = "prototopic"

type TopicActor struct {
	topic                string
	subscribers          map[subscribeIdentityStruct]*SubscriberIdentity
	subscriptionStore    KeyValueStore[*Subscribers]
	topologySubscription *eventstream.Subscription
	shouldThrottle       actor.ShouldThrottle
}

func NewTopicActor(store KeyValueStore[*Subscribers], logger *slog.Logger) *TopicActor {
	return &TopicActor{
		subscriptionStore: store,
		subscribers:       make(map[subscribeIdentityStruct]*SubscriberIdentity),
		shouldThrottle: actor.NewThrottleWithLogger(logger, 10, time.Second, func(logger *slog.Logger, count int32) {
			logger.Info("[TopicActor] Throttled logs", slog.Int("count", int(count)))
		}),
	}
}

func (t *TopicActor) Receive(c actor.Context) {
	switch msg := c.Message().(type) {
	case *actor.Started:
		t.onStarted(c)
	case *actor.Stopping:
		t.onStopping(c)
	case *actor.ReceiveTimeout:
		t.onReceiveTimeout(c)
	case *Initialize:
		t.onInitialize(c, msg)
	case *SubscribeRequest:
		t.onSubscribe(c, msg)
	case *UnsubscribeRequest:
		t.onUnsubscribe(c, msg)
	case *PubSubBatch:
		t.onPubSubBatch(c, msg)
	case *NotifyAboutFailingSubscribersRequest:
		t.onNotifyAboutFailingSubscribers(c, msg)
	case *ClusterTopology:
		t.onClusterTopologyChanged(c, msg)
	}
}

func (t *TopicActor) onStarted(c actor.Context) {
	t.topic = GetClusterIdentity(c).Identity
	t.topologySubscription = c.ActorSystem().EventStream.Subscribe(func(evt interface{}) {
		if clusterTopology, ok := evt.(*ClusterTopology); ok {
			c.Send(c.Self(), clusterTopology)
		}
	})

	sub := t.loadSubscriptions(t.topic, c.Logger())
	if sub.Subscribers != nil {
		for _, subscriber := range sub.Subscribers {
			t.subscribers[newSubscribeIdentityStruct(subscriber)] = subscriber
		}
	}
	t.unsubscribeSubscribersOnMembersThatLeft(c)

	c.Logger().Debug("Topic started", slog.String("topic", t.topic))
}

func (t *TopicActor) onStopping(c actor.Context) {
	if t.topologySubscription != nil {
		c.ActorSystem().EventStream.Unsubscribe(t.topologySubscription)
		t.topologySubscription = nil
	}
}

func (t *TopicActor) onReceiveTimeout(c actor.Context) {
	c.Stop(c.Self())
}

func (t *TopicActor) onInitialize(c actor.Context, msg *Initialize) {
	if msg.IdleTimeout != nil {
		duration := msg.IdleTimeout.AsDuration()
		if duration > 0 {
			c.SetReceiveTimeout(duration)
		}
	}
	c.Respond(&Acknowledge{})
}

type pidAndSubscriber struct {
	pid        *actor.PID
	subscriber *SubscriberIdentity
}

// onPubSubBatch handles a PubSubBatch message, sends the message to all subscribers
func (t *TopicActor) onPubSubBatch(c actor.Context, batch *PubSubBatch) {
	// map subscribers to map[address][](pid, subscriber)
	members := make(map[string][]pidAndSubscriber)
	for _, identity := range t.subscribers {
		pid := t.getPID(c, identity)
		if pid != nil {
			members[pid.Address] = append(members[pid.Address], pidAndSubscriber{pid: pid, subscriber: identity})
		}
	}

	// send message to each member
	for address, member := range members {
		subscribersOnMember := t.getSubscribersForAddress(member)
		deliveryMessage := &DeliverBatchRequest{
			Subscribers: subscribersOnMember,
			PubSubBatch: batch,
			Topic:       t.topic,
		}
		deliveryPid := actor.NewPID(address, PubSubDeliveryName)
		c.Send(deliveryPid, deliveryMessage)
	}
	c.Respond(&PublishResponse{})
}

// getSubscribersForAddress returns the subscribers for the given member list
func (t *TopicActor) getSubscribersForAddress(members []pidAndSubscriber) *Subscribers {
	subscribers := make([]*SubscriberIdentity, len(members))
	for i, member := range members {
		subscribers[i] = member.subscriber
	}
	return &Subscribers{Subscribers: subscribers}
}

// getPID returns the PID of the subscriber
func (t *TopicActor) getPID(c actor.Context, subscriber *SubscriberIdentity) *actor.PID {
	if pid := subscriber.GetPid(); pid != nil {
		return pid
	}

	return t.getClusterIdentityPid(c, subscriber.GetClusterIdentity())
}

// getClusterIdentityPid returns the PID of the clusterIdentity actor
func (t *TopicActor) getClusterIdentityPid(c actor.Context, identity *ClusterIdentity) *actor.PID {
	if identity == nil {
		return nil
	}

	return GetCluster(c.ActorSystem()).Get(identity.Identity, identity.Kind)
}

// onNotifyAboutFailingSubscribers handles a NotifyAboutFailingSubscribersRequest message
func (t *TopicActor) onNotifyAboutFailingSubscribers(c actor.Context, msg *NotifyAboutFailingSubscribersRequest) {
	t.unsubscribeUnreachablePidSubscribers(c, msg.InvalidDeliveries)
	t.logDeliveryErrors(msg.InvalidDeliveries, c.Logger())
	c.Respond(&NotifyAboutFailingSubscribersResponse{})
}

// logDeliveryErrors logs the delivery errors in one log line
func (t *TopicActor) logDeliveryErrors(reports []*SubscriberDeliveryReport, logger *slog.Logger) {
	if len(reports) > 0 || t.shouldThrottle() == actor.Open {
		subscribers := make([]string, len(reports))
		for i, report := range reports {
			subscribers[i] = report.Subscriber.String()
		}
		logger.Error("Topic following subscribers could not process the batch", slog.String("topic", t.topic), slog.String("subscribers", strings.Join(subscribers, ",")))
	}
}

// unsubscribeUnreachablePidSubscribers deletes all subscribers that have a PID that is unreachable
func (t *TopicActor) unsubscribeUnreachablePidSubscribers(_ actor.Context, allInvalidDeliveryReports []*SubscriberDeliveryReport) {
	subscribers := make([]subscribeIdentityStruct, 0, len(allInvalidDeliveryReports))
	for _, r := range allInvalidDeliveryReports {
		if r.Subscriber.GetPid() != nil && r.Status == DeliveryStatus_SubscriberNoLongerReachable {
			subscribers = append(subscribers, newSubscribeIdentityStruct(r.Subscriber))
		}
	}
	t.removeSubscribers(subscribers, nil)
}

// onClusterTopologyChanged handles a ClusterTopology message
func (t *TopicActor) onClusterTopologyChanged(ctx actor.Context, msg *ClusterTopology) {
	if len(msg.Left) > 0 {
		addressMap := make(map[string]struct{})
		for _, member := range msg.Left {
			addressMap[member.Address()] = struct{}{}
		}

		subscribersThatLeft := make([]subscribeIdentityStruct, 0, len(msg.Left))

		for identityStruct, identity := range t.subscribers {
			if pid := identity.GetPid(); pid != nil {
				if _, ok := addressMap[pid.Address]; ok {
					subscribersThatLeft = append(subscribersThatLeft, identityStruct)
				}
			}
		}
		t.removeSubscribers(subscribersThatLeft, ctx.Logger())
	}
}

// unsubscribeSubscribersOnMembersThatLeft removes subscribers that are on members that left the clusterIdentity
func (t *TopicActor) unsubscribeSubscribersOnMembersThatLeft(c actor.Context) {
	members := GetCluster(c.ActorSystem()).MemberList.Members()
	activeMemberAddresses := make(map[string]struct{})
	for _, member := range members.Members() {
		activeMemberAddresses[member.Address()] = struct{}{}
	}

	subscribersThatLeft := make([]subscribeIdentityStruct, 0)
	for s := range t.subscribers {
		if s.isPID {
			if _, ok := activeMemberAddresses[s.pid.address]; !ok {
				subscribersThatLeft = append(subscribersThatLeft, s)
			}
		}
	}
	t.removeSubscribers(subscribersThatLeft, nil)
}

// removeSubscribers remove subscribers from the topic
func (t *TopicActor) removeSubscribers(subscribersThatLeft []subscribeIdentityStruct, logger *slog.Logger) {
	if len(subscribersThatLeft) > 0 {
		for _, subscriber := range subscribersThatLeft {
			delete(t.subscribers, subscriber)
		}
		if t.shouldThrottle() == actor.Open {
			logger.Warn("Topic removed subscribers, because they are dead or they are on members that left the clusterIdentity:", slog.String("topic", t.topic), slog.Any("subscribers", subscribersThatLeft))
		}
		t.saveSubscriptionsInTopicActor(logger)
	}
}

// loadSubscriptions loads the subscriptions for the topic from the subscription store
func (t *TopicActor) loadSubscriptions(topic string, logger *slog.Logger) *Subscribers {
	// TODO: cancellation logic config?
	state, err := t.subscriptionStore.Get(context.Background(), topic)
	if err != nil {
		if t.shouldThrottle() == actor.Open {
			logger.Error("Error when loading subscriptions", slog.String("topic", topic), slog.Any("error", err))
		}
		return &Subscribers{}
	}
	if state == nil {
		return &Subscribers{}
	}
	logger.Debug("Loaded subscriptions for topic", slog.String("topic", topic), slog.Any("subscriptions", state))
	return state
}

// saveSubscriptionsInTopicActor saves the TopicActor.subscribers for the TopicActor.topic to the subscription store
func (t *TopicActor) saveSubscriptionsInTopicActor(logger *slog.Logger) {
	var subscribers *Subscribers = &Subscribers{Subscribers: maps.Values(t.subscribers)}

	// TODO: cancellation logic config?
	logger.Debug("Saving subscriptions for topic", slog.String("topic", t.topic), slog.Any("subscriptions", subscribers))
	err := t.subscriptionStore.Set(context.Background(), t.topic, subscribers)
	if err != nil && t.shouldThrottle() == actor.Open {
		logger.Error("Error when saving subscriptions", slog.String("topic", t.topic), slog.Any("error", err))
	}
}

func (t *TopicActor) onUnsubscribe(c actor.Context, msg *UnsubscribeRequest) {
	delete(t.subscribers, newSubscribeIdentityStruct(msg.Subscriber))
	t.saveSubscriptionsInTopicActor(c.Logger())
	c.Respond(&UnsubscribeResponse{})
}

func (t *TopicActor) onSubscribe(c actor.Context, msg *SubscribeRequest) {
	t.subscribers[newSubscribeIdentityStruct(msg.Subscriber)] = msg.Subscriber
	c.Logger().Debug("Topic subscribed", slog.String("topic", t.topic), slog.Any("subscriber", msg.Subscriber))
	t.saveSubscriptionsInTopicActor(c.Logger())
	c.Respond(&SubscribeResponse{})
}

// pidStruct is a struct that represents a PID
// It is used to implement the comparison interface
type pidStruct struct {
	address   string
	id        string
	requestId uint32
}

// newPIDStruct creates a new pidStruct from a *actor.PID
func newPidStruct(pid *actor.PID) pidStruct {
	return pidStruct{
		address:   pid.Address,
		id:        pid.Id,
		requestId: pid.RequestId,
	}
}

// toPID converts a pidStruct to a *actor.PID
func (p pidStruct) toPID() *actor.PID {
	return &actor.PID{
		Address:   p.address,
		Id:        p.id,
		RequestId: p.requestId,
	}
}

type clusterIdentityStruct struct {
	identity string
	kind     string
}

// newClusterIdentityStruct creates a new clusterIdentityStruct from a *ClusterIdentity
func newClusterIdentityStruct(clusterIdentity *ClusterIdentity) clusterIdentityStruct {
	return clusterIdentityStruct{
		identity: clusterIdentity.Identity,
		kind:     clusterIdentity.Kind,
	}
}

// toClusterIdentity converts a clusterIdentityStruct to a *ClusterIdentity
func (c clusterIdentityStruct) toClusterIdentity() *ClusterIdentity {
	return &ClusterIdentity{
		Identity: c.identity,
		Kind:     c.kind,
	}
}

// subscriberIdentityStruct is a struct that represents a SubscriberIdentity
// It is used to implement the comparison interface
type subscribeIdentityStruct struct {
	isPID           bool
	pid             pidStruct
	clusterIdentity clusterIdentityStruct
}

// newSubscriberIdentityStruct creates a new subscriberIdentityStruct from a *SubscriberIdentity
func newSubscribeIdentityStruct(subscriberIdentity *SubscriberIdentity) subscribeIdentityStruct {
	if subscriberIdentity.GetPid() != nil {
		return subscribeIdentityStruct{
			isPID: true,
			pid:   newPidStruct(subscriberIdentity.GetPid()),
		}
	}
	return subscribeIdentityStruct{
		isPID:           false,
		clusterIdentity: newClusterIdentityStruct(subscriberIdentity.GetClusterIdentity()),
	}
}

// toSubscriberIdentity converts a subscribeIdentityStruct to a *SubscriberIdentity
func (s subscribeIdentityStruct) toSubscriberIdentity() *SubscriberIdentity {
	if s.isPID {
		return &SubscriberIdentity{
			Identity: &SubscriberIdentity_Pid{Pid: s.pid.toPID()},
		}
	}
	return &SubscriberIdentity{
		Identity: &SubscriberIdentity_ClusterIdentity{ClusterIdentity: s.clusterIdentity.toClusterIdentity()},
	}
}
