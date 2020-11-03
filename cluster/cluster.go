package cluster

import (
	"time"

	"github.com/AsynkronIT/gonet"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type Cluster struct {
	actorSystem    *actor.ActorSystem
	config         *ClusterConfig
	remote         *remote.Remote
	pidCache       *pidCacheValue
	memberList     *memberListValue
	partitionValue *partitionValue
}

func NewCluster(actorSystem *actor.ActorSystem, config *ClusterConfig) *Cluster {
	return &Cluster{
		actorSystem: actorSystem,
		config:      config,
	}
}

func (c *Cluster) Start() {
	cfg := c.config
	c.remote = remote.NewRemote(c.actorSystem, c.config.RemoteConfig)

	// TODO: make it possible to become a cluster even if remoting is already started
	c.remote.Start()

	address := c.actorSystem.ProcessRegistry.Address
	h, p := gonet.GetAddress(address)
	plog.Info("Starting Proto.Actor cluster", log.String("address", address))
	kinds := c.remote.GetKnownKinds()

	// for each known kind, spin up a partition-kind actor to handle all requests for that kind
	c.partitionValue = setupPartition(c, kinds)
	c.pidCache = setupPidCache(c.actorSystem)
	c.memberList = setupMemberList(c)

	_ = cfg.ClusterProvider.RegisterMember(cfg.Name, h, p, kinds, cfg.InitialMemberStatusValue, cfg.MemberStatusValueSerializer)
	cfg.ClusterProvider.MonitorMemberStatusChanges()
}

func (c *Cluster) Shutdown(graceful bool) {
	if graceful {
		_ = c.config.ClusterProvider.Shutdown()
		// This is to wait ownership transferring complete.
		time.Sleep(time.Millisecond * 2000)
		c.memberList.stopMemberList()
		c.pidCache.stopPidCache()
		c.partitionValue.stopPartition()
	}

	c.remote.Shutdown(graceful)

	address := c.actorSystem.ProcessRegistry.Address
	plog.Info("Stopped Proto.Actor cluster", log.String("address", address))
}

// Get a PID to a virtual actor
func (c *Cluster) Get(name string, kind string) (*actor.PID, remote.ResponseStatusCode) {
	// Check Cache
	if pid, ok := c.pidCache.getCache(name); ok {
		return pid, remote.ResponseStatusCodeOK
	}

	// Get Pid
	address := c.memberList.getPartitionMember(name, kind)
	if address == "" {
		// No available member found
		return nil, remote.ResponseStatusCodeUNAVAILABLE
	}

	// package the request as a remote.ActorPidRequest
	req := &remote.ActorPidRequest{
		Kind: kind,
		Name: name,
	}

	// ask the DHT partition for this name to give us a PID
	remotePartition := c.partitionValue.partitionForKind(address, kind)
	r, err := c.actorSystem.Root.RequestFuture(remotePartition, req, c.config.TimeoutTime).Result()
	if err == actor.ErrTimeout {
		plog.Error("PidCache Pid request timeout")
		return nil, remote.ResponseStatusCodeTIMEOUT
	} else if err != nil {
		plog.Error("PidCache Pid request error", log.Error(err))
		return nil, remote.ResponseStatusCodeERROR
	}

	response, ok := r.(*remote.ActorPidResponse)
	if !ok {
		return nil, remote.ResponseStatusCodeERROR
	}

	statusCode := remote.ResponseStatusCode(response.StatusCode)
	switch statusCode {
	case remote.ResponseStatusCodeOK:
		// save cache
		c.pidCache.addCache(name, response.Pid)
		// tell the original requester that we have a response
		return response.Pid, statusCode
	default:
		// forward to requester
		return response.Pid, statusCode
	}
}
