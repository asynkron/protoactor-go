package partition_identity

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	clustering "github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/log"
	"time"
)

type GrainMeta struct {
	ID  *clustering.ClusterIdentity
	PID *actor.PID
}

type spawnTask func() *clustering.ActivationResponse

type identityActor struct {
	cluster          *clustering.Cluster
	partitionManager *PartitionManager
	lookup           map[string]*actor.PID
	spawns           map[string]spawnTask
	topologyHash     uint64
	handoverTimeout  time.Duration
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
}

func (p *identityActor) onStopped() {
	plog.Info("Stopped PartitionIdentity")
}

func (p *identityActor) onActivationRequest(msg *clustering.ActivationRequest, ctx actor.Context) {

}

func (p *identityActor) onActivationTerminated(msg *clustering.ActivationTerminated, ctx actor.Context) {

}

func (p *identityActor) onClusterTopology(msg *clustering.ClusterTopology, ctx actor.Context) {

}

func (p *identityActor) spawn(msg *clustering.ActivationRequest, context actor.Context) {
	if p.cluster.MemberList.Length() <= 0 {
		context.Respond(&clustering.ActivationResponse{Pid: nil})
		plog.Error("spawn failed: Empty memberlist")
		return
	}

}

func (p *identityActor) spawning(spawningPID *actor.PID, msg *clustering.ActivationRequest, context actor.Context, retryCount int) {

}

func (p *identityActor) spawningCallback(req *clustering.ActivationRequest, ctx actor.Context, key string, resp interface{}, err error) {

}
