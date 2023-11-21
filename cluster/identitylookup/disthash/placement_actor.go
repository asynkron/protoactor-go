package disthash

import (
	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"log/slog"
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
	case *actor.Started:
		ctx.Logger().Info("Placement actor started")
	case *actor.Stopping:
		ctx.Logger().Info("Placement actor stopping")
		p.onStopping(ctx)
	case *actor.Stopped:
		ctx.Logger().Info("Placement actor stopped")
	case *actor.Terminated:
		p.onTerminated(msg, ctx)
	case *clustering.ActivationRequest:
		p.onActivationRequest(msg, ctx)
	case *clustering.ClusterTopology:
		p.onClusterTopology(msg, ctx)
	default:
		ctx.Logger().Error("Invalid message", slog.Any("message", msg), slog.Any("sender", ctx.Sender()))
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
			ctx.Logger().Error("Failed to poison actor", slog.String("identity", key), slog.Any("error", err))
		}
	}
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
	if clusterKind == nil {
		ctx.Logger().Error("Unknown cluster kind", slog.String("kind", msg.ClusterIdentity.Kind))

		// TODO: what to do here?
		ctx.Respond(nil)
		return
	}

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

func (p *placementActor) onClusterTopology(msg *clustering.ClusterTopology, ctx actor.Context) {
	rdv := clustering.NewRendezvous()
	rdv.UpdateMembers(msg.Members)
	myAddress := p.cluster.ActorSystem.Address()
	for identity, meta := range p.actors {
		ownerAddress := rdv.GetByIdentity(identity)
		if ownerAddress == myAddress {

			ctx.Logger().Debug("Actor stays", slog.String("identity", identity), slog.String("owner", ownerAddress), slog.String("me", myAddress))
			continue
		}

		ctx.Logger().Debug("Actor moved", slog.String("identity", identity), slog.String("owner", ownerAddress), slog.String("me", myAddress))

		ctx.Poison(meta.PID)
	}
}
