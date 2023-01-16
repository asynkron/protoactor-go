package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/remote"
	"sync"
	"time"
)

var pubsubMemberDeliveryLogThrottle = actor.NewThrottle(10, time.Second, func(i int32) {
	plog.Warn("[PubSubMemberDeliveryActor] Throttled logs", log.Int("count", int(i)))
})

type PubSubMemberDeliveryActor struct {
	subscriberTimeout time.Duration
}

func NewPubSubMemberDeliveryActor(subscriberTimeout time.Duration) *PubSubMemberDeliveryActor {
	return &PubSubMemberDeliveryActor{
		subscriberTimeout: subscriberTimeout,
	}
}

func (p *PubSubMemberDeliveryActor) Receive(c actor.Context) {
	if batch, ok := c.Message().(*DeliverBatchRequest); ok {
		topicBatch := &PubSubAutoRespondBatch{Envelopes: batch.PubSubBatch.Envelopes}
		siList := batch.Subscribers.Subscribers

		invalidDeliveries := make([]*SubscriberDeliveryReport, 0, len(siList))
		var lock sync.Mutex

		var wg sync.WaitGroup
		for _, identity := range siList {
			wg.Add(1)
			go func(identity *SubscriberIdentity) {
				defer wg.Done()
				report := p.DeliverBatch(c, topicBatch, identity) // generally concurrent safe, depends on the implementation of cluster.Call and actor.RequestFuture
				if report.Status != DeliveryStatus_Delivered {
					lock.Lock()
					invalidDeliveries = append(invalidDeliveries, report)
					lock.Unlock()
				}
			}(identity)
		}

		if len(invalidDeliveries) > 0 {
			cluster := GetCluster(c.ActorSystem())
			// we use cluster.Call to locate the topic actor in the cluster
			_, _ = cluster.Call(batch.Topic, TopicActorKind, &NotifyAboutFailingSubscribersRequest{InvalidDeliveries: invalidDeliveries})
		}

	}
}

// DeliverBatch delivers PubSubAutoRespondBatch to SubscriberIdentity.
func (p *PubSubMemberDeliveryActor) DeliverBatch(c actor.Context, batch *PubSubAutoRespondBatch, s *SubscriberIdentity) *SubscriberDeliveryReport {
	status := DeliveryStatus_OtherError
	if pid := s.GetPid(); pid != nil {
		status = p.DeliverToPid(c, batch, pid)
	}
	if ci := s.GetClusterIdentity(); ci != nil {
		status = p.DeliverToClusterIdentity(c, batch, ci)
	}
	return &SubscriberDeliveryReport{
		Subscriber: s,
		Status:     status,
	}
}

// DeliverToPid delivers PubSubAutoRespondBatch to PID.
func (p *PubSubMemberDeliveryActor) DeliverToPid(c actor.Context, batch *PubSubAutoRespondBatch, pid *actor.PID) DeliveryStatus {
	_, err := c.RequestFuture(pid, batch, p.subscriberTimeout).Result()
	if err != nil {
		switch err {
		case actor.ErrTimeout, remote.ErrTimeout:
			if pubsubMemberDeliveryLogThrottle() == actor.Open {
				plog.Warn("Pub-sub message delivered to pid timed out", log.String("pid", pid.String()))
			}
			return DeliveryStatus_Timeout
		case actor.ErrDeadLetter, remote.ErrDeadLetter:
			if pubsubMemberDeliveryLogThrottle() == actor.Open {
				plog.Warn("Pub-sub message cannot be delivered to pid as it is no longer available", log.String("pid", pid.String()))
			}
			return DeliveryStatus_SubscriberNoLongerReachable
		default:
			if pubsubMemberDeliveryLogThrottle() == actor.Open {
				plog.Warn("Error while delivering pub-sub message to pid", log.String("pid", pid.String()), log.Error(err))
			}
			return DeliveryStatus_OtherError
		}
	}
	return DeliveryStatus_Delivered
}

// DeliverToClusterIdentity delivers PubSubAutoRespondBatch to ClusterIdentity.
func (p *PubSubMemberDeliveryActor) DeliverToClusterIdentity(c actor.Context, batch *PubSubAutoRespondBatch, ci *ClusterIdentity) DeliveryStatus {
	cluster := GetCluster(c.ActorSystem())
	// deliver to virtual actor
	// delivery should always be possible, since a virtual actor always exists
	response, err := cluster.Call(ci.Identity, ci.Kind, batch, WithTimeout(p.subscriberTimeout))
	if err != nil {
		switch err {
		case actor.ErrTimeout, remote.ErrTimeout:
			if pubsubMemberDeliveryLogThrottle() == actor.Open {
				plog.Warn("Pub-sub message delivered to cluster identity timed out", log.String("cluster identity", ci.String()))
			}
			return DeliveryStatus_Timeout
		case actor.ErrDeadLetter, remote.ErrDeadLetter:
			if pubsubMemberDeliveryLogThrottle() == actor.Open {
				plog.Warn("Pub-sub message cannot be delivered to cluster identity as it is no longer available", log.String("cluster identity", ci.String()))
			}
			return DeliveryStatus_SubscriberNoLongerReachable
		default:
			if pubsubMemberDeliveryLogThrottle() == actor.Open {
				plog.Warn("Error while delivering pub-sub message to cluster identity", log.String("cluster identity", ci.String()), log.Error(err))
			}
			return DeliveryStatus_OtherError
		}
	}
	if response == nil {
		if pubsubMemberDeliveryLogThrottle() == actor.Open {
			plog.Warn("Pub-sub message delivered to cluster identity timed out", log.String("cluster identity", ci.String()))
		}
		return DeliveryStatus_Timeout
	}
	return DeliveryStatus_Delivered
}
