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

	c.context = NewDefaultClusterContext(c)
	c.PidCache = NewPidCache()
	c.MemberList = NewMemberList(c)
	c.subscribeToTopologyEvents()

	return c
}

func (c *Cluster) subscribeToTopologyEvents() {
	c.ActorSystem.EventStream.Subscribe(func(evt interface{}) {
		if clusterTopology, ok := evt.(*ClusterTopology); ok {
			for _, member := range clusterTopology.Left {
				c.PidCache.RemoveByMember(member)
			}
		}
	})
}

func (c *Cluster) Id() extensions.ExtensionId {
	return extensionId
}

func GetCluster(actorSystem *actor.ActorSystem) *Cluster {
	c := actorSystem.Extensions.Get(extensionId)
	return c.(*Cluster)
}

func (c *Cluster) StartMember() {
	cfg := c.Config
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)
	for kind, props := range cfg.Kinds {
		c.Remote.Register(kind, props)
	}

	// TODO: make it possible to become a cluster even if remoting is already started
	c.Remote.Start()

	address := c.ActorSystem.Address()
	plog.Info("Starting Proto.Actor cluster member", log.String("address", address))

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
