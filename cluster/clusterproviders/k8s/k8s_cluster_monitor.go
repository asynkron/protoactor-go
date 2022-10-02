package k8s

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/scheduler"
)

type k8sClusterMonitorActor struct {
	*Provider
	actor.Behavior

	refreshCanceller scheduler.CancelFunc
}

func (kcm *k8sClusterMonitorActor) Receive(ctx actor.Context) { kcm.Behavior.Receive(ctx) }

func (kcm *k8sClusterMonitorActor) init(ctx actor.Context) {
	switch r := ctx.Message().(type) {
	case *RegisterMember:
		// make sure timeout is set to some meaningful value
		timeout := getTimeout(ctx, kcm)

		if err := kcm.registerMember(timeout); err != nil {
			plog.Error("Failed to register service to k8s, will retry", log.Error(err))
			ctx.Send(ctx.Self(), r)
			return
		}
		plog.Info("Registered service to k8s")
	case *DeregisterMember:
		plog.Debug("Deregistering service from k8s")
		timeout := getTimeout(ctx, kcm)

		if err := kcm.deregisterMember(timeout); err != nil {
			plog.Error("Failed to deregister service from k8s, proceeding with shutdown", log.Error(err))
		} else {
			plog.Info("Deregistered service from k8s")
		}
		ctx.Respond(&DeregisterMemberResponse{})
	case *StartWatchingCluster:
		if err := kcm.startWatchingCluster(); err != nil {
			plog.Error("Failed to start watching k8s cluster, will retry", log.Error(err))
			ctx.Send(ctx.Self(), r)
			return
		}
		plog.Info("k8s cluster started to being watched")
	case *StopWatchingCluster:
		if kcm.cancelWatch != nil {
			kcm.cancelWatch()
		}
		ctx.Respond(&StopWatchingClusterResponse{})
	}
}

func getTimeout(ctx actor.Context, kcm *k8sClusterMonitorActor) time.Duration {
	timeout := ctx.ReceiveTimeout()
	if timeout.Microseconds() == 0 {
		timeout = kcm.Provider.cluster.Config.RequestTimeoutTime
		if timeout.Microseconds() == 0 {
			timeout = time.Second * 5 // default to 5 seconds
		}
	}

	return timeout
}

// creates and initializes a new k8sClusterMonitorActor in the heap and
// returns a reference to its memory address
func newClusterMonitor(provider *Provider) actor.Actor {
	kcm := k8sClusterMonitorActor{
		Behavior: actor.NewBehavior(),
		Provider: provider,
	}
	kcm.Become(kcm.init)
	return &kcm
}
