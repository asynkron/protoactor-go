package partition_identity

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	clustering "github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/log"

	"time"
)

type spawnTask func() *clustering.ActivationResponse

// This actor is responsible to keep track of identities owned by this member
// it does not manage the cluster spawned actors itself, only identity->remote PID management
// TLDR; this is a partition/bucket in the distributed hash table which makes up the identity lookup
//
// for spawning/activating cluster actors see PartitionActivator.cs

type identityActor struct {
	cluster          *clustering.Cluster
	partitionManager *PartitionManager
	lookup           map[string]*actor.PID
	spawns           map[string]spawnTask
	topologyHash     uint64
	handoverTimeout  time.Duration
	rdv              *clustering.RendezvousV2
}

func newIdentityActor(c *clustering.Cluster, p *PartitionManager) *identityActor {
	return &identityActor{
		cluster:          c,
		partitionManager: p,
		handoverTimeout:  10 * time.Second,
		lookup:           map[string]*actor.PID{},
	}
}

func (p *identityActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		p.onStart(ctx)
	case *actor.Stopped:
		p.onStopped()
	case *clustering.ActivationRequest:
		p.onActivationRequest(msg, ctx)
	case *clustering.ActivationTerminated:
		p.onActivationTerminated(msg, ctx)
	case *clustering.ClusterTopology:
		p.onClusterTopology(msg, ctx)
	default:
		plog.Error("Invalid message", log.TypeOf("type", msg), log.PID("sender", ctx.Sender()))
	}
}

func (p *identityActor) onStart(ctx actor.Context) {
	plog.Debug("Started PartitionIdentity")
	self := ctx.Self()
	ctx.ActorSystem().EventStream.Subscribe(func(evt interface{}) {
		if at, ok := evt.(clustering.ActivationTerminated); ok {
			p.cluster.ActorSystem.Root.Send(self, at)
		}
	})
}

func (p *identityActor) onStopped() {
	plog.Info("Stopped PartitionIdentity")
}

func (p *identityActor) onActivationRequest(msg *clustering.ActivationRequest, ctx actor.Context) {
	ownerAddress := p.rdv.Get(msg.ClusterIdentity.AsKey())

	// should I own it?
	if ownerAddress != ctx.Self().Address {
		ownerPid := p.partitionManager.PidOfIdentityActor(ownerAddress)
		ctx.Forward(ownerPid)
		return
	}

	// do I already own it?
	if pid, ok := p.lookup[msg.ClusterIdentity.AsKey()]; ok {
		response := &clustering.ActivationResponse{
			Pid: pid,
		}

		ctx.Respond(response)
		return
	}

	// Get activator
	activatorAddress := p.cluster.MemberList.GetActivatorMember(msg.ClusterIdentity.Kind)
	activator := p.partitionManager.PidOfActivatorActor(activatorAddress)

	// No activator found, bail out and respond empty
	if activator == nil {
		//TODO: log
		response := &clustering.ActivationResponse{
			Pid: nil,
		}
		ctx.Respond(response)
		return
	}

	//TODO: continue here
}

func (p *identityActor) onActivationTerminated(msg *clustering.ActivationTerminated, ctx actor.Context) {
	// //we get this via broadcast to all nodes, remove if we have it, or ignore
	key := msg.ClusterIdentity.AsKey()
	_, ok := p.spawns[key]
	if ok {
		return
	}

	// Logger.LogDebug("[PartitionIdentityActor] Terminated {Pid}", msg.Pid);
	p.cluster.PidCache.RemoveByValue(msg.ClusterIdentity.Identity, msg.ClusterIdentity.Kind, msg.Pid)
	delete(p.lookup, key)
}

func (p *identityActor) onClusterTopology(msg *clustering.ClusterTopology, ctx actor.Context) {
	// await _cluster.MemberList.TopologyConsensus();
	if p.topologyHash == msg.TopologyHash {
		return
	}

	members := msg.Members
	p.rdv = clustering.NewRendezvousV2(members)
	p.lookup = map[string]*actor.PID{}
	futures := make([]*actor.Future, 0)

	requestMsg := &clustering.IdentityHandoverRequest{
		TopologyHash: msg.TopologyHash,
		Address:      ctx.Self().Address,
	}

	for _, m := range members {
		placementPid := p.partitionManager.PidOfActivatorActor(m.Address())
		future := ctx.RequestFuture(placementPid, requestMsg, 5*time.Second)

		futures = append(futures, future)
	}

	for _, f := range futures {
		res, _ := f.Result()
		if response, ok := res.(clustering.IdentityHandoverResponse); ok {
			for _, activation := range response.Actors {
				p.takeOwnership(activation)
			}
		}
	}
}

func (p *identityActor) spawn(msg *clustering.ActivationRequest, context actor.Context) {
	if p.cluster.MemberList.Length() <= 0 {
		context.Respond(&clustering.ActivationResponse{Pid: nil})
		plog.Error("spawn failed: Empty memberlist")
		return
	}

}

func (p *identityActor) takeOwnership(activation *clustering.Activation) {
	key := activation.ClusterIdentity.AsKey()
	if existing, ok := p.lookup[key]; ok {
		if existing.Address == activation.Pid.Address {
			return
		}
	}

	p.lookup[key] = activation.Pid
}
