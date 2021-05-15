package partition_identity

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	clustering "github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/log"
)

type placementActor struct {
	cluster          *clustering.Cluster
	partitionManager *PartitionManager
	actors           map[string]GrainMeta
}

func newPlacementActor(c *clustering.Cluster, pm *PartitionManager) *placementActor {
	return &placementActor{
		cluster:          c,
		partitionManager: pm,
		actors:           map[string]GrainMeta{},
	}
}

func (p *placementActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Terminated:
		p.onTerminated(msg, ctx)
	case *clustering.IdentityHandoverRequest:
		p.onIdentityHandoverRequest(msg, ctx)
	case *clustering.ActivationRequest:
		p.onActivationRequest(msg, ctx)
	default:
		plog.Error("Invalid message", log.TypeOf("type", msg), log.PID("sender", ctx.Sender()))
	}
}

func (p *placementActor) onTerminated(msg *actor.Terminated, ctx actor.Context) {
	found, key, meta := p.pidToMeta(msg.Who)

	activationTerminated := &clustering.ActivationTerminated{
		Pid:             msg.Who,
		ClusterIdentity: meta.ID,
	}
	p.partitionManager.cluster.MemberList.BroadcastEvent(activationTerminated, true)

	if found {
		delete(p.actors, *key)
	}
}

// this is pure, we do not change any state or actually move anything
// the requester also provide its own view of the world in terms of members
// TLDR; we are not using any topology state from this actor itself
func (p *placementActor) onIdentityHandoverRequest(msg *clustering.IdentityHandoverRequest, ctx actor.Context) {
	count := 0
	response := &clustering.IdentityHandoverResponse{}
	requestAddress := ctx.Sender().Address
	rdv := clustering.NewRendezvousV2(msg.Members)
	for identity, meta := range p.actors {
		// who owns this identity according to the requesters memberlist?
		ownerAddress := rdv.Get(identity)
		// this identity is not owned by the requester
		if ownerAddress != requestAddress {
			continue
		}
		// _logger.LogDebug("Transfer {Identity} to {newOwnerAddress} -- {TopologyHash}", clusterIdentity, ownerAddress,
		// msg.TopologyHash
		// );

		actorToHandOver := &clustering.Activation{
			ClusterIdentity: meta.ID,
			Pid:             meta.PID,
		}

		response.Actors = append(response.Actors, actorToHandOver)
		count++
	}

	plog.Debug("Transferred ownership to other members", log.Int("count", count))
	ctx.Respond(response)
}

func (p *placementActor) onActivationRequest(msg *clustering.ActivationRequest, ctx actor.Context) {
	key := msg.ClusterIdentity.AsKey()
	meta, found := p.actors[key]
	if found {
		response := &clustering.ActivationResponse{
			Pid: meta.PID,
		}
		ctx.Respond(response)
		return
	}

	clusterKindProps := p.cluster.GetClusterKind(msg.ClusterIdentity.Kind)

	//TODO: wrap in WithClusterIdentity

	pid := ctx.SpawnPrefix(clusterKindProps, msg.ClusterIdentity.Identity)

	p.actors[key] = GrainMeta{
		ID:  msg.ClusterIdentity,
		PID: pid,
	}

	response := &clustering.ActivationResponse{
		Pid: pid,
	}

	ctx.Respond(response)
}

func (p *placementActor) pidToMeta(pid *actor.PID) (bool, *string, *GrainMeta) {
	for k, v := range p.actors {
		if v.PID == pid {
			return true, &k, &v
		}
	}
	return false, nil, nil
}
