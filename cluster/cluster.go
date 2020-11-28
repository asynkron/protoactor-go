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
	ActorSystem    *actor.ActorSystem
	Config         *Config
	remote         *remote.Remote
	pidCache       *pidCacheValue
	MemberList     *memberListValue
	partitionValue *partitionValue
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
	for kind, props := range c.Config.Kinds {
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

	if err := cfg.ClusterProvider.StartMember(c); err != nil {
		panic(err)
	}
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
	}

	c.remote.Shutdown(graceful)

	address := c.ActorSystem.Address()
	plog.Info("Stopped Proto.Actor cluster", log.String("address", address))
}

// Get a PID to a virtual actor
func (c *Cluster) Get(name string, kind string) (*actor.PID, remote.ResponseStatusCode) {
	// Check Cache
	if pid, ok := c.pidCache.getCache(name); ok {
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
		Name: name,
	}

	// ask the DHT partition for this name to give us a PID
	remotePartition := c.partitionValue.partitionForKind(address, kind)
	r, err := c.ActorSystem.Root.RequestFuture(remotePartition, req, c.Config.TimeoutTime).Result()
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

// GetClusterKinds Get kinds of virtual actor
func (c *Cluster) GetClusterKinds() []string {
	if c.remote == nil {
		return nil
	}
	return c.remote.GetKnownKinds()
}

// RequestFuture just call context.RequestFuture with retries.
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

		timeout := _callopts.Timeout
		_resp, err := _context.RequestFuture(pid, msg, timeout).Result()
		if err != nil {
			plog.Error("cluster.RequestFuture failed", log.Error(err))
			lastError = err
			switch err {
			case actor.ErrTimeout, remote.ErrTimeout:
				_callopts.RetryAction(i)
			case remote.ErrDeadLetter: // TODO: not implemented yet
				_callopts.RetryAction(i)
			default:
				return nil, err
			}
		}
		return _resp, nil
	}
	return nil, lastError
}
