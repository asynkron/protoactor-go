package cluster

import (
	"time"

	"github.com/AsynkronIT/gonet"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type Cluster struct {
	actorSystem *actor.ActorSystem
	config      *ClusterConfig
	remote      *remote.Remote
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
	setupPartition(kinds)
	setupPidCache()
	setupMemberList()

	cfg.ClusterProvider.RegisterMember(cfg.Name, h, p, kinds, cfg.InitialMemberStatusValue, cfg.MemberStatusValueSerializer)
	cfg.ClusterProvider.MonitorMemberStatusChanges()
}

func (c *Cluster) Shutdown(graceful bool) {
	if graceful {
		c.config.ClusterProvider.Shutdown()
		// This is to wait ownership transferring complete.
		time.Sleep(time.Millisecond * 2000)
		stopMemberList()
		stopPidCache()
		stopPartition()
	}

	c.remote.Shutdown(graceful)

	address := c.actorSystem.ProcessRegistry.Address
	plog.Info("Stopped Proto.Actor cluster", log.String("address", address))
}

// Get a PID to a virtual actor
func (c *Cluster) Get(name string, kind string) (*actor.PID, remote.ResponseStatusCode) {
	// Check Cache
	if pid, ok := pidCache.getCache(name); ok {
		return pid, remote.ResponseStatusCodeOK
	}

	// Get Pid
	address := memberList.getPartitionMember(name, kind)
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
	remotePartition := partition.partitionForKind(address, kind)
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
		pidCache.addCache(name, response.Pid)
		// tell the original requester that we have a response
		return response.Pid, statusCode
	default:
		// forward to requester
		return response.Pid, statusCode
	}
}

// GetMemberPIDs returns PIDs of members for the specified kind
func GetMemberPIDs(kind string) actor.PIDSet {
	pids := actor.PIDSet{}
	if memberList == nil {
		return pids
	}

	memberList.mutex.RLock()
	defer memberList.mutex.RUnlock()

	for _, value := range memberList.members {
		for _, memberKind := range value.Kinds {
			if kind == memberKind {
				pids.Add(actor.NewPID(value.Address(), kind))
			}
		}
	}
	return pids
}

// RemoveCache at PidCache
func RemoveCache(name string) {
	pidCache.removeCacheByName(name)
}
