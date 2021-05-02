package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/extensions"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
	"time"
)

var extensionId = extensions.NextExtensionId()

type Cluster struct {
	ActorSystem    *actor.ActorSystem
	Config         *Config
	Remote         *remote.Remote
	PidCache       *pidCacheValue
	MemberList     *MemberList
	IdentityLookup IdentityLookup
	kinds          map[string]*actor.Props
	context        ClusterContext
}

func New(actorSystem *actor.ActorSystem, config *Config) *Cluster {
	c := &Cluster{
		ActorSystem: actorSystem,
		Config:      config,
	}

	actorSystem.Extensions.Register(c)
	//TODO subscribe to eventstream and clear pid cache
	//SubscribeToTopologyEvents()

	return c
}

/*
private void SubscribeToTopologyEvents() =>
	System.EventStream.Subscribe<ClusterTopology>(e => {
			System.Metrics.Get<ClusterMetrics>().ClusterTopologyEventGauge.Set(e.MemberSet.Count,
				new[] {System.Id, System.Address, e.TopologyHash().ToString()}
			);

			foreach (var member in e.Left)
			{
				PidCache.RemoveByMember(member);
			}
		}
	);
*/

func (c *Cluster) Id() extensions.ExtensionId {
	return extensionId
}

func GetCluster(actorSystem *actor.ActorSystem) *Cluster {
	c := actorSystem.Extensions.Get(extensionId)
	return c.(*Cluster)
}

func (c *Cluster) Start() {
	cfg := c.Config
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)
	for kind, props := range cfg.Kinds {
		c.Remote.Register(kind, props)
	}

	// TODO: make it possible to become a cluster even if remoting is already started
	c.Remote.Start()

	address := c.ActorSystem.Address()
	plog.Info("Starting Proto.Actor cluster", log.String("address", address))

	// for each known kind, spin up a partition-kind actor to handle all requests for that kind
	c.PidCache = setupPidCache(c.ActorSystem)
	c.MemberList = NewMemberList(c)
	c.IdentityLookup.Setup(c, c.GetClusterKinds(), false)

	if err := cfg.ClusterProvider.StartMember(c); err != nil {
		panic(err)
	}
	time.Sleep(1 * time.Second)
}

func (c *Cluster) GetClusterKinds() []string {
	keys := make([]string, 0, len(c.kinds))
	for k := range c.kinds {
		keys = append(keys, k)
	}
	return keys
}

func (c *Cluster) StartClient() {
	cfg := c.Config
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)

	c.Remote.Start()

	address := c.ActorSystem.Address()
	plog.Info("Starting Proto.Actor cluster-client", log.String("address", address))

	c.PidCache = setupPidCache(c.ActorSystem)
	c.MemberList = NewMemberList(c)
	c.IdentityLookup.Setup(c, c.GetClusterKinds(), true)

	if err := cfg.ClusterProvider.StartClient(c); err != nil {
		panic(err)
	}
}

func (c *Cluster) Shutdown(graceful bool) {

	if graceful {
		_ = c.Config.ClusterProvider.Shutdown(graceful)
		c.IdentityLookup.Shutdown()
	}

	c.Remote.Shutdown(graceful)

	address := c.ActorSystem.Address()
	plog.Info("Stopped Proto.Actor cluster", log.String("address", address))
}

func (c *Cluster) Get(identity string, kind string) *actor.PID {
	return c.IdentityLookup.Get(NewClusterIdentity(identity, kind))
}

func (c *Cluster) Request(identity string, kind string, message interface{}) (interface{}, error) {
	return c.context.Request(identity, kind, message)
}

func (c *Cluster) GetClusterKind(kind string) *actor.Props {
	props, ok := c.Config.Kinds[kind]
	if !ok {
		plog.Error("Invalid kind", log.String("kind", kind))
		return nil
	}
	return props
}
