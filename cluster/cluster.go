package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/extensions"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var extensionId = extensions.NextExtensionId()

type Cluster struct {
	ActorSystem      *actor.ActorSystem
	Config           *Config
	remote           *remote.Remote
	pidCache         *pidCacheValue
	MemberList       *MemberList
	partitionValue   *partitionValue
	partitionManager *PartitionManager
}

func New(actorSystem *actor.ActorSystem, config *Config) *Cluster {
	c := &Cluster{
		ActorSystem: actorSystem,
		Config:      config,
	}

	actorSystem.Extensions.Register(c)

	return c
}

func (c *Cluster) Id() extensions.ExtensionId {
	return extensionId
}

func GetCluster(actorSystem *actor.ActorSystem) *Cluster {
	c := actorSystem.Extensions.Get(extensionId)
	return c.(*Cluster)
}

func (c *Cluster) Start() {
	cfg := c.Config
	c.remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)
	for kind, props := range cfg.Kinds {
		c.remote.Register(kind, props)
	}

	// TODO: make it possible to become a cluster even if remoting is already started
	c.remote.Start()

	address := c.ActorSystem.Address()
	plog.Info("Starting Proto.Actor cluster", log.String("address", address))
	kinds := c.remote.GetKnownKinds()

	// for each known kind, spin up a partition-kind actor to handle all requests for that kind
	c.partitionValue = setupPartition(c, kinds)
	c.pidCache = setupPidCache(c.ActorSystem)
	c.MemberList = setupMemberList(c)
	c.partitionManager = newPartitionManager(c)
	c.partitionManager.Start()

	if err := cfg.ClusterProvider.StartMember(c); err != nil {
		panic(err)
	}
	time.Sleep(1 * time.Second)
}

func (c *Cluster) StartClient() {
	cfg := c.Config
	c.remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)

	c.remote.Start()

	address := c.ActorSystem.Address()
	plog.Info("Starting Proto.Actor cluster-client", log.String("address", address))
	kinds := c.remote.GetKnownKinds()

	// for each known kind, spin up a partition-kind actor to handle all requests for that kind
	c.partitionValue = setupPartition(c, kinds)
	c.pidCache = setupPidCache(c.ActorSystem)
	c.MemberList = setupMemberList(c)
	c.partitionManager = newPartitionManager(c)
	c.partitionManager.Start()

	if err := cfg.ClusterProvider.StartClient(c); err != nil {
		panic(err)
	}
}

func (c *Cluster) Shutdown(graceful bool) {
	if graceful {
		_ = c.Config.ClusterProvider.Shutdown(graceful)
		// This is to wait ownership transferring complete.
		time.Sleep(time.Millisecond * 2000)
		c.MemberList.stopMemberList()
		c.pidCache.stopPidCache()
		c.partitionValue.stopPartition()
		c.partitionManager.Stop()
	}

	c.remote.Shutdown(graceful)

	address := c.ActorSystem.Address()
	plog.Info("Stopped Proto.Actor cluster", log.String("address", address))
}

// Get a PID to a virtual actor
func (c *Cluster) GetV1(name string, kind string) (*actor.PID, remote.ResponseStatusCode) {
	// Check Cache
	clusterActorId := kind + "/" + name
	if pid, ok := c.pidCache.getCache(clusterActorId); ok {
		return pid, remote.ResponseStatusCodeOK
	}

	// Get Pid
	address := c.MemberList.getPartitionMember(name, kind)
	if address == "" {
		// No available member found
		return nil, remote.ResponseStatusCodeUNAVAILABLE
	}

	// package the request as a remote.ActorPidRequest
	req := &remote.ActorPidRequest{
		Kind: kind,
		Name: clusterActorId,
	}

	// ask the DHT partition for this name to give us a PID
	remotePartition := c.partitionValue.partitionForKind(address, kind)
	plog.Error("PidCache Pid request ", log.String("remote", remotePartition.String()))
	r, err := c.ActorSystem.Root.RequestFuture(remotePartition, req, c.Config.TimeoutTime).Result()
	if err != nil {
		if err == actor.ErrTimeout {
			plog.Error("PidCache Pid request timeout", log.String("remote", remotePartition.String()))
			return nil, remote.ResponseStatusCodeTIMEOUT
		}
		plog.Error("PidCache Pid request error", log.Error(err), log.String("remote", remotePartition.String()))
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
		c.pidCache.addCache(clusterActorId, response.Pid)
		// tell the original requester that we have a response
		return response.Pid, statusCode
	default:
		// forward to requester
		return response.Pid, statusCode
	}
}

// Get a PID to a virtual actor
func (c *Cluster) Get(name string, kind string) (*actor.PID, remote.ResponseStatusCode) {
	// Check Cache
	grainId := ClusterIdentity{Kind: kind, Identity: name}
	clusterActorId := grainId.AsKey()
	if pid, ok := c.pidCache.getCache(clusterActorId); ok {
		return pid, remote.ResponseStatusCodeOK
	}

	ownerAddr := c.MemberList.getPartitionMemberV2(&grainId)
	if ownerAddr == "" {
		return nil, remote.ResponseStatusCodeUNAVAILABLE
	}

	// package the request as a remote.ActorPidRequest
	req := &ActivationRequest{
		ClusterIdentity: &grainId,
		RequestId:       "",
	}

	system := c.ActorSystem
	ownerPid := c.partitionManager.PidOfIdentityActor(kind, ownerAddr)
	if ownerPid == nil {
		return nil, remote.ResponseStatusCodeUNAVAILABLE
	}
	// ask the DHT partition for this name to give us a PID
	r, err := system.Root.RequestFuture(ownerPid, req, c.Config.TimeoutTime).Result()
	if err != nil {
		if err == actor.ErrTimeout {
			plog.Error("PidCache Pid request timeout", log.String("pid", ownerPid.String()))
			return nil, remote.ResponseStatusCodeTIMEOUT
		}
		plog.Error("PidCache Pid request error", log.String("pid", ownerPid.String()), log.Error(err))
		return nil, remote.ResponseStatusCodeERROR
	}
	if r == nil {
		plog.Debug("activation request failed: no response")
		return nil, remote.ResponseStatusCodeERROR
	}
	switch resp := r.(type) {
	case *ActivationResponse:
		statusCode := remote.ResponseStatusCode(resp.StatusCode)
		if resp.Pid == nil {
			if statusCode == 0 {
				// FIXME: print reason would be better.
				statusCode = remote.ResponseStatusCodeERROR
			}
			plog.Debug("activation request failed", log.PID("from", ownerPid), log.String("status", statusCode.String()))
			return resp.Pid, statusCode
		}
		c.pidCache.addCache(clusterActorId, resp.Pid)
		return resp.Pid, statusCode
	default:
		plog.Debug("activation request failed: invalid response", log.TypeOf("type", r), log.PID("from", ownerPid))
		return nil, remote.ResponseStatusCodeERROR
	}
}

// GetClusterKinds Get kinds of virtual actor
func (c *Cluster) GetClusterKinds() []string {
	if c.remote == nil {
		plog.Debug("remote is nil")
		return nil
	}
	return c.remote.GetKnownKinds()
}

// Call is a wrap of context.RequestFuture with retries.
func (c *Cluster) Call(name string, kind string, msg interface{}, callopts ...*GrainCallOptions) (interface{}, error) {
	var _callopts *GrainCallOptions = nil
	if len(callopts) > 0 {
		_callopts = callopts[0]
	} else {
		_callopts = DefaultGrainCallOptions(c)
	}

	_context := c.ActorSystem.Root
	var lastError error
	for i := 0; i < _callopts.RetryCount; i++ {
		pid, statusCode := c.Get(name, kind)
		if statusCode != remote.ResponseStatusCodeOK && statusCode != remote.ResponseStatusCodePROCESSNAMEALREADYEXIST {
			lastError = statusCode.AsError()
			if statusCode == remote.ResponseStatusCodeTIMEOUT {
				_callopts.RetryAction(i)
				continue
			}
			return nil, statusCode.AsError()
		}
		if pid == nil {
			return nil, remote.ErrUnknownError
		}

		timeout := _callopts.Timeout
		_resp, err := _context.RequestFuture(pid, msg, timeout).Result()
		if err != nil {
			plog.Error("cluster.RequestFuture failed", log.Error(err), log.PID("pid", pid))
			lastError = err
			switch err {
			case actor.ErrTimeout, remote.ErrTimeout:
				_callopts.RetryAction(i)
				id := ClusterIdentity{Kind: kind, Identity: name}
				c.pidCache.removeCacheByName(id.AsKey())
				continue
			case actor.ErrDeadLetter, remote.ErrDeadLetter:
				_callopts.RetryAction(i)
				id := ClusterIdentity{Kind: kind, Identity: name}
				c.pidCache.removeCacheByName(id.AsKey())
				continue
			default:
				return nil, err
			}
		}
		return _resp, nil
	}
	return nil, lastError
}

func (c *Cluster) GetClusterKind(kind string) *actor.Props {
	props, ok := c.Config.Kinds[kind]
	if !ok {
		plog.Error("Invalid kind", log.String("kind", kind))
		return nil
	}
	return props
}
