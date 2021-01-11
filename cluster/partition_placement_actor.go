package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster/chash"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type partitionPlacementActor struct {
	cluster      *Cluster
	_actors      map[string]GrainMeta
	chash        chash.ConsistentHash
	spawns       map[string]spawnTask
	lastEventId  uint64
	partionKind  *PartitionKind
	self         *actor.PID
	logPartition log.Field
}

func newPartitionPlacementActor(c *Cluster, pk *PartitionKind, _chash chash.ConsistentHash) *partitionPlacementActor {
	return &partitionPlacementActor{
		cluster:      c,
		partionKind:  pk,
		chash:        _chash,
		_actors:      map[string]GrainMeta{},
		logPartition: log.String("partition", pk.actorNames.Placement),
	}
}

func (p *partitionPlacementActor) PID() *actor.PID {
	return p.self
}

func (p *partitionPlacementActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		p.onStart(ctx)
	case *actor.Stopped:
		p.onStopped(ctx)
	case *actor.ReceiveTimeout:
		p.onTimeout(msg, ctx)
	case *actor.Stopping:
		// nothing
	case *remote.Ping:
		ctx.Respond(&remote.Pong{})

	case *actor.Terminated:
		p.handleTerminated(msg, ctx)
	case *ClusterTopology:
		p.handlerClusterTopology(msg, ctx)
	case *IdentityHandoverRequest:
		p.handleIdentityHandoverRequest(msg, ctx)
	case *ActivationRequest:
		p.handleActivationRequest(msg, ctx)
	// case *ActivationTerminated:
	// 	p.handleActivationTerminated(msg, ctx)
	default:
		plog.Error("Invalid message", p.logPartition, log.TypeOf("type", msg), log.PID("sender", ctx.Sender()))
	}
}

func (p *partitionPlacementActor) onStart(ctx actor.Context) {
	plog.Debug("Started PartitionPlacement", p.logPartition)
	p.self = ctx.Self()
}

func (p *partitionPlacementActor) onStopped(ctx actor.Context) {
	plog.Info("Stopped PartitionPlacement", p.logPartition)
}

func (p *partitionPlacementActor) onTimeout(msg *actor.ReceiveTimeout, ctx actor.Context) {
	ctx.SetReceiveTimeout(3 * time.Second)
	plog.Info("...")
}

func (p *partitionPlacementActor) handleTerminated(msg *actor.Terminated, ctx actor.Context) {
	plog.Debug("handleTerminated", p.logPartition)
	var req *ActivationTerminated

	var actorKey string
	if baseIdLength := len(p.self.Id); baseIdLength+1 < len(msg.Who.Id) {
		actorKey = msg.Who.Id[baseIdLength+1:]
		if meta, ok := p._actors[actorKey]; ok {
			req = &ActivationTerminated{
				Pid:             meta.PID,
				ClusterIdentity: meta.ID,
				EventId:         meta.EventID,
			}
		}
	}

	if req == nil {
		plog.Warn("handleTerminated", p.logPartition, log.String("status", "lookup slowly"), log.PID("who", msg.Who), log.String("grain", actorKey))
		for _, meta := range p._actors {
			if meta.PID.Equal(msg.Who) {
				req = &ActivationTerminated{
					Pid:             meta.PID,
					ClusterIdentity: meta.ID,
					EventId:         meta.EventID,
				}
				actorKey = meta.ID.AsKey()
				break
			}
		}
	}

	if req == nil {
		plog.Warn("handleTerminated", p.logPartition, log.String("status", "not found"), log.PID("who", msg.Who), log.String("grain", actorKey))
		return
	}
	delete(p._actors, actorKey)

	ownerAddr := p.chash.Get(req.ClusterIdentity.Identity)
	ownerPid := p.partionKind.PidOfIdentityActor(ownerAddr)
	ctx.Send(ownerPid, req)
	plog.Debug("handleTerminated", p.logPartition, log.String("status", "OK"), log.String("grain", actorKey))
}

func (p *partitionPlacementActor) handleIdentityHandoverRequest(msg *IdentityHandoverRequest, ctx actor.Context) {
	now := time.Now()
	p.chash = NewRendezvousV2(msg.Members)
	resp := IdentityHandoverResponse{}
	for actorKey, meta := range p._actors {
		fromAddr := msg.Address
		ownerAddr := p.chash.Get(actorKey)
		if fromAddr != ownerAddr {
			continue
		}
		resp.Actors = append(resp.Actors, &Activation{
			Pid:             meta.PID,
			ClusterIdentity: meta.ID,
			EventId:         meta.EventID,
		})
	}
	ctx.Respond(&resp)
	plog.Debug("handleIdentityHandoverRequest", p.logPartition, log.Duration("cost", time.Since(now)))

}

func (p *partitionPlacementActor) handlerClusterTopology(msg *ClusterTopology, ctx actor.Context) {
	if p.lastEventId >= msg.EventId {
		plog.Debug("skip ClusterTopology", log.Uint64("eventId", msg.EventId), log.Int("members", len(msg.Members)))
		return
	}
	p.lastEventId = msg.EventId
	p.chash = NewRendezvousV2(msg.Members)
	return
}

func (p *partitionPlacementActor) handleActivationRequest(msg *ActivationRequest, ctx actor.Context) {
	key := msg.ClusterIdentity.AsKey()
	_log := plog.With(p.logPartition, log.String("grainId", key))
	_log.Debug("handleActivationRequest")
	if meta, isExist := p._actors[key]; isExist {
		ctx.Respond(&ActivationResponse{Pid: meta.PID})
		_log.Debug("handleActivationRequest", log.String("status", "cache hited"))
		return
	}
	_log.Debug("handleActivationRequest", log.String("status", "cache missing"))
	props := p.cluster.GetClusterKind(msg.ClusterIdentity.Kind)
	if props == nil {
		ctx.Respond(&ActivationResponse{Pid: nil, StatusCode: uint32(remote.ResponseStatusCodeERROR)})
		_log.Debug("handleActivationRequest", log.String("status", "no props"))
		return
	}
	props.WithSpawnFunc(func(system *actor.ActorSystem, id string, props *actor.Props, parentContext actor.SpawnerContext) (*actor.PID, error) {
		pid, err := actor.DefaultSpawner(system, id, props, parentContext)
		if pid != nil {
			ctx.Send(pid, &ClusterInit{
				ID:   msg.ClusterIdentity.Identity,
				Kind: msg.ClusterIdentity.Kind,
			})
			_log.Debug("handleActivationRequest", log.String("status", "send ClusterInit message."))
		}
		return pid, err
	})
	pid, err := ctx.SpawnNamed(props, key)
	if err != nil && err != actor.ErrNameExists {
		_log.Error("handleActivationRequest", log.String("status", "spawn failed"), log.Error(err), log.Stack())
	}
	ctx.Respond(&ActivationResponse{Pid: pid})
	if pid != nil {
		p._actors[key] = GrainMeta{ID: msg.ClusterIdentity, PID: pid}
	}
	_log.Debug("handleActivationRequest", log.String("status", "spawn OK"))
	return
}
