package partition

import (
	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/log"
)

type GrainMeta struct {
	ID  *clustering.ClusterIdentity
	PID *actor.PID
}

type placementActor struct {
	cluster          *clustering.Cluster
	partitionManager *Manager
	actors           map[string]GrainMeta
}

func newPlacementActor(c *clustering.Cluster, pm *Manager) *placementActor {
	return &placementActor{
		cluster:          c,
		partitionManager: pm,
		actors:           map[string]GrainMeta{},
	}
}

func (p *placementActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Stopping:
		plog.Info("Placement actor stopping")
		p.onStopping(ctx)
	case *actor.Stopped:
		plog.Info("Placement actor stopped")
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

func (p *placementActor) onStopping(ctx actor.Context) {
	futures := make(map[string]*actor.Future, len(p.actors))

	for key, meta := range p.actors {
		futures[key] = ctx.PoisonFuture(meta.PID)
	}

	for key, future := range futures {
		err := future.Wait()
		if err != nil {
			plog.Error("Failed to poison actor", log.String("identity", key), log.Error(err))
		}
	}
}

// this is pure, we do not change any state or actually move anything
// the requester also provide its own view of the world in terms of members
// TLDR; we are not using any topology state from this actor itself
func (p *placementActor) onIdentityHandoverRequest(msg *clustering.IdentityHandoverRequest, ctx actor.Context) {
	count := 0
	response := &clustering.IdentityHandover{}
	requestAddress := ctx.Sender().Address
	rdv := clustering.NewRendezvous()
	rdv.UpdateMembers(msg.CurrentTopology.Members)
	for identity, meta := range p.actors {
		// who owns this identity according to the requesters memberlist?
		ownerAddress := rdv.GetByIdentity(identity)
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

	clusterKind := p.cluster.GetClusterKind(msg.ClusterIdentity.Kind)

	props := clustering.WithClusterIdentity(clusterKind.Props, msg.ClusterIdentity)

	pid := ctx.SpawnPrefix(props, msg.ClusterIdentity.Identity)

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
