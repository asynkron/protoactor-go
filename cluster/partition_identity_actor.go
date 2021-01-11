package cluster

import (
	"sync"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster/chash"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
	cmap "github.com/orcaman/concurrent-map"
)

type GrainMeta struct {
	ID      *ClusterIdentity
	PID     *actor.PID
	EventID uint64
}

type spawnTask func() *ActivationResponse

type partitionIdentityActor struct {
	cluster               *Cluster
	partitionKind         *PartitionKind
	lookup                cmap.ConcurrentMap
	chash                 chash.ConsistentHash
	spawns                map[string]spawnTask
	lastEventId           uint64
	lastEventTimestamp    time.Time
	handoverTimeout       time.Duration
	topologyChangeTimeout time.Duration
	self                  *actor.PID
	spawnings             map[string]*spawningProcess // spawning actor/grain futures
	logPartition          log.Field
}

func newPartitionIdentityActor(c *Cluster, pk *PartitionKind, rdv chash.ConsistentHash) *partitionIdentityActor {
	return &partitionIdentityActor{
		cluster:               c,
		partitionKind:         pk,
		handoverTimeout:       10 * time.Second,
		topologyChangeTimeout: 3 * time.Second,
		lookup:                cmap.New(),
		spawnings:             map[string]*spawningProcess{},
		chash:                 rdv,
		logPartition:          log.String("partition", pk.actorNames.Identity),
	}
}

func (p *partitionIdentityActor) PID() *actor.PID {
	return p.self
}

func (p *partitionIdentityActor) Receive(ctx actor.Context) {
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

	case *ActivationRequest:
		p.handleActivationRequest(msg, ctx)
	case *ActivationTerminated:
		p.handleActivationTerminated(msg, ctx)
	case *ClusterTopology:
		p.handleClusterTopology(msg, ctx)
	default:
		plog.Error("Invalid message", p.logPartition, log.TypeOf("type", msg), log.PID("sender", ctx.Sender()))
	}
}

func (p *partitionIdentityActor) onStart(ctx actor.Context) {
	plog.Debug("Started PartitionIdentity", p.logPartition)
	p.lastEventTimestamp = time.Now()
	p.self = ctx.Self()
}

func (p *partitionIdentityActor) onStopped(ctx actor.Context) {
	plog.Info("Stopped PartitionIdentity", p.logPartition)
}

func (p *partitionIdentityActor) onTimeout(msg *actor.ReceiveTimeout, ctx actor.Context) {
	ctx.SetReceiveTimeout(3 * time.Second)
	plog.Info(" ...", p.logPartition)
}

func (p *partitionIdentityActor) handleActivationRequest(msg *ActivationRequest, ctx actor.Context) {
	grainId := msg.ClusterIdentity
	key := grainId.AsKey()
	_log := plog.With(p.logPartition, log.String("grain", key))
	if p.chash == nil {
		_log.Debug("handleActivationRequest", log.String("status", "nil DHT"))
		ctx.Respond(&ActivationResponse{Pid: nil})
		return
	}
	// other member
	ownerAddr := p.chash.Get(grainId.Identity)
	if ownerAddr != p.self.Address {
		ownerPID := p.partitionKind.PidOfIdentityActor(ownerAddr)
		ctx.Forward(ownerPID)
		_log.Debug("handleActivationRequest", log.PID("forwardTo", ownerPID))
		return
	}

	// self
	if _pid, ok := p.lookup.Get(key); ok {
		_log.Debug("handleActivationRequest", log.String("status", "cache hited"))
		meta := _pid.(GrainMeta)
		ctx.Respond(&ActivationResponse{Pid: meta.PID})
		return
	}
	_log.Debug("handleActivationRequest", log.String("status", "cache missing, spawning"))
	p.spawn(msg, ctx)
}

func (p *partitionIdentityActor) handleActivationTerminated(msg *ActivationTerminated, ctx actor.Context) {
	// clean cache
	key := msg.ClusterIdentity.AsKey()
	p.lookup.Remove(key)

	ownerAddr := p.chash.Get(msg.ClusterIdentity.Identity)
	if ownerAddr != p.self.Address {
		ownerPid := p.partitionKind.PidOfIdentityActor(ownerAddr)
		ctx.Forward(ownerPid)
		plog.Debug("Terminated", p.logPartition, log.String("owner", ownerAddr), log.String("grain", key))
		return
	}
	plog.Debug("Terminated", p.logPartition, log.String("owner", "self"), log.String("grain", key))
}

func (p *partitionIdentityActor) handleClusterTopology(msg *ClusterTopology, ctx actor.Context) {
	if p.lastEventId >= msg.EventId {
		plog.Warn("Skipped ClusterTopology", log.String("kind", p.partitionKind.Kind), log.Uint64("eventId", msg.EventId), log.Int("members", len(msg.Members)))
		return
	}
	now := time.Now()
	p.lastEventId = msg.EventId
	p.chash = NewRendezvousV2(msg.Members)
	p.lookup = cmap.New()
	var req = IdentityHandoverRequest{
		EventId: msg.EventId,
		Members: msg.Members,
		Address: p.self.Address,
	}
	var wg sync.WaitGroup
	for _, member := range msg.Members {
		wg.Add(1)
		placementPid := p.partitionKind.PidOfPlacementActor(member.Address())
		go func() {
			defer wg.Done()
			_resp, err := ctx.RequestFuture(placementPid, &req, p.handoverTimeout).Result()
			if err != nil {
				plog.Error("Invalid IdentityHandoverResponse", p.logPartition, log.PID("placement", placementPid), log.Error(err))
				return
			}
			switch resp := _resp.(type) {
			case *IdentityHandoverResponse:
				p.takeOwnership(resp)
			default:
				plog.Error("Invalid IdentityHandoverResponse", p.logPartition, log.TypeOf("type", msg), log.PID("from", placementPid))
			}
		}()
	}
	wg.Wait()
	plog.Info("Updated ClusterTopology",
		log.Uint64("eventId", msg.EventId),
		log.Int("members", len(msg.Members)),
		log.String("kind", p.partitionKind.Kind),
		log.Duration("cost", time.Since(now)))
	return
}

func (p *partitionIdentityActor) takeOwnership(resp *IdentityHandoverResponse) {
	for _, tmp := range resp.Actors {
		key := tmp.ClusterIdentity.AsKey()
		if old, isExist := p.lookup.Get(key); isExist {
			_old := old.(GrainMeta)
			if _old.PID.Address == tmp.Pid.Address {
				continue
			}
		}
		p.lookup.Set(key, GrainMeta{
			ID:      tmp.ClusterIdentity,
			PID:     tmp.Pid,
			EventID: tmp.EventId,
		})
	}
}

func (p *partitionIdentityActor) spawn(msg *ActivationRequest, context actor.Context) {
	if p.cluster.MemberList.Length() <= 0 {
		context.Respond(&ActivationResponse{Pid: nil})
		plog.Error("spawn failed: Empty memberlist")
		return
	}
	key := msg.ClusterIdentity.AsKey()
	plog.Debug("spawn", log.String("grain", key))

	if ownerAddr := p.chash.Get(msg.ClusterIdentity.Identity); ownerAddr != p.self.Address {
		pid := p.partitionKind.PidOfIdentityActor(ownerAddr)
		context.Forward(pid)
		return
	}

	spawnCallback := func(r interface{}, err error) {
		delete(p.spawnings, key)
		// Check if exist in current partition dictionary
		// This is necessary to avoid race condition during partition map transferring.
		if _meta, ok := p.lookup.Get(key); ok {
			if meta, ok := _meta.(GrainMeta); ok {
				context.Respond(&ActivationResponse{Pid: meta.PID})
			} else {
				plog.Error("Invalid GrainMeta", log.TypeOf("type", meta))
				context.Respond(&ActivationResponse{Pid: nil})
				p.lookup.Remove(key)
			}
			return
		}
		p.spawningCallback(msg, context, key, r, err)
		return
	}

	// Check if is spawning, if so just await spawning finish.
	if spawning := p.spawnings[key]; spawning != nil {
		context.AwaitFuture(spawning.Future, spawnCallback)
		return
	}

	// Create SpawningProcess and cache it in spawnings dictionary.
	spawning := &spawningProcess{actor.NewFuture(context.ActorSystem(), -1), ""}
	p.spawnings[key] = spawning

	// Await SpawningProcess
	context.AwaitFuture(spawning.Future, spawnCallback)
	// Perform Spawning
	go p.spawning(spawning.PID(), msg, context, 3)
}

func (p *partitionIdentityActor) spawning(spawningPID *actor.PID, msg *ActivationRequest, context actor.Context, retryCount int) {
	// for i := 0; i < retryCount; i++ {
	// ownerAddr := p.cluster.MemberList.getPartitionMemberV2(msg.ClusterIdentity)
	ownerAddr := p.chash.Get(msg.ClusterIdentity.Identity)
	if ownerAddr == "" {
		context.Send(spawningPID, &ActivationResponse{Pid: nil})
		plog.Debug("Empty address of owner", log.PID("spawningPID", spawningPID), log.String("address", ownerAddr))
		return
	}
	timeout := p.cluster.Config.TimeoutTime
	pid := p.partitionKind.PidOfPlacementActor(ownerAddr)
	plog.Debug("spawning", log.PID("pid", pid), log.PID("spawningPID", spawningPID))
	context.RequestFuture(pid, msg, timeout).PipeTo(spawningPID)
	// }
}

func (p *partitionIdentityActor) spawningCallback(req *ActivationRequest, ctx actor.Context, key string, resp interface{}, err error) {
	plog.Debug("spawning callback", log.String("key", key), log.Object("resp", resp), log.Error(err))
	if resp == nil {
		ctx.Respond(&ActivationResponse{Pid: nil})
		return
	}
	var respPID *actor.PID
	switch _resp := resp.(type) {
	case *ActivationResponse:
		ctx.Respond(_resp)
		respPID = _resp.Pid
	default:
		ctx.Respond(&ActivationResponse{Pid: nil})
	}
	if respPID != nil {
		p.lookup.Set(key, GrainMeta{PID: respPID, ID: req.ClusterIdentity})
	}
	return
}
