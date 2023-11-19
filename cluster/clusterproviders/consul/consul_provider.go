package consul

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/hashicorp/consul/api"
)

var ProviderShuttingDownError = fmt.Errorf("consul cluster provider is shutting down")

type Provider struct {
	cluster            *cluster.Cluster
	deregistered       bool
	shutdown           bool
	id                 string
	clusterName        string
	address            string
	port               int
	knownKinds         []string
	index              uint64 // consul blocking index
	client             *api.Client
	ttl                time.Duration
	refreshTTL         time.Duration
	updateTTLWaitGroup sync.WaitGroup
	deregisterCritical time.Duration
	blockingWaitTime   time.Duration
	clusterError       error
	pid                *actor.PID
	consulConfig       *api.Config
}

func New(opts ...Option) (*Provider, error) {
	return NewWithConfig(&api.Config{}, opts...)
}

func NewWithConfig(consulConfig *api.Config, opts ...Option) (*Provider, error) {
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, err
	}
	p := &Provider{
		client:             client,
		ttl:                3 * time.Second,
		refreshTTL:         1 * time.Second,
		deregisterCritical: 60 * time.Second,
		blockingWaitTime:   20 * time.Second,
		consulConfig:       consulConfig,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (p *Provider) init(c *cluster.Cluster) error {
	knownKinds := c.GetClusterKinds()
	clusterName := c.Config.Name
	memberId := c.ActorSystem.ID

	host, port, err := c.ActorSystem.GetHostPort()
	if err != nil {
		return err
	}

	p.cluster = c
	p.id = memberId
	p.clusterName = clusterName
	p.address = host
	p.port = port
	p.knownKinds = knownKinds
	return nil
}

func (p *Provider) StartMember(c *cluster.Cluster) error {
	err := p.init(c)
	if err != nil {
		return err
	}

	p.pid, err = c.ActorSystem.Root.SpawnNamed(actor.PropsFromProducer(func() actor.Actor {
		return newProviderActor(p)
	}), "consul-provider")
	if err != nil {
		p.cluster.Logger().Error("Failed to start consul-provider actor", slog.Any("error", err))
		return err
	}

	return nil
}

func (p *Provider) StartClient(c *cluster.Cluster) error {
	if err := p.init(c); err != nil {
		return err
	}
	p.blockingStatusChange()
	p.monitorMemberStatusChanges()
	return nil
}

func (p *Provider) DeregisterMember() error {
	err := p.deregisterService()
	if err != nil {
		fmt.Println(err)
		return err
	}
	p.deregistered = true
	return nil
}

func (p *Provider) Shutdown(graceful bool) error {
	if p.shutdown {
		return nil
	}
	p.shutdown = true
	if p.pid != nil {
		if err := p.cluster.ActorSystem.Root.StopFuture(p.pid).Wait(); err != nil {
			p.cluster.Logger().Error("Failed to stop consul-provider actor", slog.Any("error", err))
		}
		p.pid = nil
	}

	return nil
}

func blockingUpdateTTL(p *Provider) error {
	p.clusterError = p.client.Agent().UpdateTTL("service:"+p.id, "", api.HealthPassing)
	return p.clusterError
}

func (p *Provider) registerService() error {
	s := &api.AgentServiceRegistration{
		ID:      p.id,
		Name:    p.clusterName,
		Tags:    p.knownKinds,
		Address: p.address,
		Port:    p.port,
		Meta: map[string]string{
			"id": p.id,
		},
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: p.deregisterCritical.String(),
			TTL:                            p.ttl.String(),
		},
	}
	return p.client.Agent().ServiceRegister(s)
}

func (p *Provider) deregisterService() error {
	return p.client.Agent().ServiceDeregister(p.id)
}

// call this directly after registering the service
func (p *Provider) blockingStatusChange() {
	p.notifyStatuses()
}

func (p *Provider) notifyStatuses() {
	statuses, meta, err := p.client.Health().Service(p.clusterName, "", false, &api.QueryOptions{
		WaitIndex: p.index,
		WaitTime:  p.blockingWaitTime,
	})
	p.cluster.Logger().Info("Consul health check")

	if err != nil {
		p.cluster.Logger().Error("notifyStatues", slog.Any("error", err))
		return
	}
	p.index = meta.LastIndex

	var members []*cluster.Member
	for _, v := range statuses {
		if len(v.Checks) > 0 && v.Checks.AggregatedStatus() == api.HealthPassing {
			memberId := v.Service.Meta["id"]
			if memberId == "" {
				memberId = fmt.Sprintf("%v@%v:%v", p.clusterName, v.Service.Address, v.Service.Port)
				p.cluster.Logger().Info("meta['id'] was empty, fixeds", slog.String("id", memberId))
			}
			members = append(members, &cluster.Member{
				Id:    memberId,
				Host:  v.Service.Address,
				Port:  int32(v.Service.Port),
				Kinds: v.Service.Tags,
			})
		}
	}
	// the reason why we want this in a batch and not as individual messages is that
	// if we have an atomic batch, we can calculate what nodes have left the cluster
	// passing events one by one, we can't know if someone left or just haven't changed status for a long time

	// publish the current cluster topology onto the event stream
	p.cluster.MemberList.UpdateClusterTopology(members)
}

func (p *Provider) monitorMemberStatusChanges() {
	go func() {
		for !p.shutdown {
			p.notifyStatuses()
		}
	}()
}
