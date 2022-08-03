package consul

import (
	"fmt"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/scheduler"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

type providerActor struct {
	*Provider
	actor.Behavior
	refreshCanceller scheduler.CancelFunc
}

type (
	RegisterService   struct{}
	UpdateTTL         struct{}
	MemberListUpdated struct {
		members []*cluster.Member
		index   uint64
	}
)

func (pa *providerActor) Receive(ctx actor.Context) {
	pa.Behavior.Receive(ctx)
}

func newProviderActor(provider *Provider) actor.Actor {
	pa := &providerActor{
		Behavior: actor.NewBehavior(),
		Provider: provider,
	}
	pa.Become(pa.init)
	return pa
}

func (pa *providerActor) init(ctx actor.Context) {
	switch ctx.Message().(type) {
	case *actor.Started:
		ctx.Send(ctx.Self(), &RegisterService{})
	case *RegisterService:
		if err := pa.registerService(); err != nil {
			plog.Error("Failed to register service to consul, will retry", log.Error(err))
			ctx.Send(ctx.Self(), &RegisterService{})
		} else {
			plog.Info("Registered service to consul")
			refreshScheduler := scheduler.NewTimerScheduler(ctx)
			pa.refreshCanceller = refreshScheduler.SendRepeatedly(0, pa.refreshTTL, ctx.Self(), &UpdateTTL{})
			if err := pa.startWatch(ctx); err == nil {
				pa.Become(pa.running)
			}
		}
	}
}

func (pa *providerActor) running(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *UpdateTTL:
		if err := blockingUpdateTTL(pa.Provider); err != nil {
			plog.Warn("Failed to update TTL", log.Error(err))
		}
	case *MemberListUpdated:
		pa.cluster.MemberList.UpdateClusterTopology(msg.members)
	case *actor.Stopping:
		pa.refreshCanceller()
		if err := pa.deregisterService(); err != nil {
			plog.Error("Failed to deregister service from consul", log.Error(err))
		} else {
			plog.Info("De-registered service from consul")
		}
	}
}

func (pa *providerActor) startWatch(ctx actor.Context) error {
	params := make(map[string]interface{})
	params["type"] = "service"
	params["service"] = pa.clusterName
	params["passingonly"] = false
	plan, err := watch.Parse(params)
	if err != nil {
		plog.Error("Failed to parse consul watch definition", log.Error(err))
		return err
	}
	plan.Handler = func(index uint64, result interface{}) {
		pa.processConsulUpdate(index, result, ctx)
	}

	go func() {
		if err = plan.RunWithConfig(pa.consulServerAddress, pa.consulConfig); err != nil {
			plog.Error("Failed to start consul watch", log.Error(err))
			panic(err)
		}
	}()

	return nil
}

func (pa *providerActor) processConsulUpdate(index uint64, result interface{}, ctx actor.Context) {
	serviceEntries, ok := result.([]*api.ServiceEntry)
	if !ok {
		plog.Warn("Didn't get expected data from consul watch")
		return
	}
	var members []*cluster.Member
	for _, v := range serviceEntries {
		if len(v.Checks) > 0 && v.Checks.AggregatedStatus() == api.HealthPassing {
			memberId := v.Service.Meta["id"]
			if memberId == "" {
				memberId = fmt.Sprintf("%v@%v:%v", pa.clusterName, v.Service.Address, v.Service.Port)
				plog.Info("meta['id'] was empty, fixed", log.String("id", memberId))
			}
			members = append(members, &cluster.Member{
				Id:    memberId,
				Host:  v.Service.Address,
				Port:  int32(v.Service.Port),
				Kinds: v.Service.Tags,
			})
		}
	}

	// delay the fist update until there is at least one member
	if len(members) > 0 {
		ctx.Send(ctx.Self(), &MemberListUpdated{
			members: members,
			index:   index,
		})
	}
}
