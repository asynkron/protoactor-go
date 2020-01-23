package consul

import (
	"fmt"
	"time"

	"github.com/otherview/protoactor-go/remote"
	"github.com/hashicorp/consul/api"
	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/log"
	"github.com/otherview/protoactor-go/eventstream"

)

var (
	plog = log.New(log.DebugLevel, "[CLUSTER] [CONSUL]")
)

type ConsulProvider struct {
	deregistered          bool
	shutdown              bool
	id                    string
	clusterName           string
	address               string
	port                  int
	knownKinds            []string
	index                 uint64 // consul blocking index
	client                *api.Client
	ttl                   time.Duration
	refreshTTL            time.Duration
	deregisterCritical    time.Duration
	blockingWaitTime      time.Duration
	statusValue           cluster.MemberStatusValue
	statusValueSerializer cluster.MemberStatusValueSerializer
	clusterError          error
}

func New() (*ConsulProvider, error) {
	return NewWithConfig(&api.Config{},
		60 * time.Second,
		20 * time.Second,
	)
}

func NewWithConfig(consulConfig *api.Config, deregisterCritical time.Duration, blockingWaitTime time.Duration) (*ConsulProvider, error) {
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, err
	}
	p := &ConsulProvider{
		client:             client,
		ttl:                3 * time.Second,
		refreshTTL:         1 * time.Second,
		deregisterCritical: deregisterCritical,
		blockingWaitTime:   blockingWaitTime,
	}
	return p, nil
}

func (p *ConsulProvider) RegisterMember(clusterName string, address string, port int, knownKinds []string,
	statusValue cluster.MemberStatusValue, serializer cluster.MemberStatusValueSerializer) error {
	p.id = fmt.Sprintf("%v@%v:%v", clusterName, address, port)
	p.clusterName = clusterName
	p.address = address
	p.port = port
	p.knownKinds = knownKinds
	p.statusValue = statusValue
	p.statusValueSerializer = serializer

	err := p.registerService()
	if err != nil {
		return err
	}

	// IMPORTANT: do these ops sync directly after registering.
	// this will ensure that the local node sees its own information upon startup.

	// force our own TTL to be OK
	err = p.blockingUpdateTTL()
	if err != nil {
		return err
	}

	// force our own existence to be part of the first status update
	p.blockingStatusChange()

	p.UpdateTTL()
	return nil
}

func (p *ConsulProvider) DeregisterMember() error {
	err := p.deregisterService()
	if err != nil {
		fmt.Println(err)
		return err
	}
	p.deregistered = true
	return nil
}

func (p *ConsulProvider) Shutdown() error {
	p.shutdown = true
	if !p.deregistered {
		err := p.DeregisterMember()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *ConsulProvider) UpdateTTL() {
	go func() {
		for !p.shutdown {

			newKinds := remote.GetKnownKinds()
			if len(newKinds) != len(p.knownKinds) {
				p.knownKinds = newKinds
				p.registerService()
			}

			err := p.blockingUpdateTTL()
			if err == nil {
				time.Sleep(p.refreshTTL)
				continue
			}

			plog.Error("Failure refreshing service TTL. Trying to reregister service if not in consul.", log.Error(err))

			services, err := p.client.Agent().Services()
			for id := range services {
				if id == p.id {
					plog.Info("Service found in consul -> doing nothing")
					time.Sleep(p.refreshTTL)
					continue
				}
			}

			err = p.registerService()
			if err != nil {
				plog.Error("Error reregistering service with consul", log.Error(err))
				time.Sleep(p.refreshTTL)
				continue
			}

			plog.Info("Reregistered service in consul")
			time.Sleep(p.refreshTTL)
		}
	}()
}

func (p *ConsulProvider) UpdateMemberStatusValue(statusValue cluster.MemberStatusValue) error {
	p.statusValue = statusValue
	if p.statusValue == nil {
		return nil
	}
	// Register service again to update the status value
	return p.registerService()
}

func (p *ConsulProvider) blockingUpdateTTL() error {
	p.clusterError = p.client.Agent().UpdateTTL("service:"+p.id, "", api.HealthPassing)
	return p.clusterError
}

func (p *ConsulProvider) registerService() error {
	s := &api.AgentServiceRegistration{
		ID:      p.id,
		Name:    p.clusterName,
		Tags:    p.knownKinds,
		Address: p.address,
		Port:    p.port,
		Meta: map[string]string{
			"StatusValue": p.statusValueSerializer.Serialize(p.statusValue),
		},
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: p.deregisterCritical.String(),
			TTL:                            p.ttl.String(),
		},
	}
	return p.client.Agent().ServiceRegister(s)
}

func (p *ConsulProvider) deregisterService() error {
	return p.client.Agent().ServiceDeregister(p.id)
}

// call this directly after registering the service
func (p *ConsulProvider) blockingStatusChange() {
	p.notifyStatuses()
}

func (p *ConsulProvider) notifyStatuses() {
	statuses, meta, err := p.client.Health().Service(p.clusterName, "", true, &api.QueryOptions{
		WaitIndex: p.index,
		WaitTime:  p.blockingWaitTime,
	})
	if err != nil {
		plog.Error("Error getting the services health from consul", log.Error(err))
		time.Sleep(p.refreshTTL)
		return
	}
	p.index = meta.LastIndex

	res := make(cluster.ClusterTopologyEvent, len(statuses))
	for i, v := range statuses {
		key := fmt.Sprintf("%v/%v:%v", p.clusterName, v.Service.Address, v.Service.Port)
		memberID := key
		memberStatusVal := p.statusValueSerializer.Deserialize(v.Node.Meta["StatusValue"])
		ms := &cluster.MemberStatus{
			MemberID:    memberID,
			Host:        v.Service.Address,
			Port:        v.Service.Port,
			Kinds:       v.Service.Tags,
			Alive:       len(v.Checks) > 0 && v.Checks.AggregatedStatus() == api.HealthPassing,
			StatusValue: memberStatusVal,
		}
		res[i] = ms

		// Update Tags for this member
		if memberID == p.id {
			p.knownKinds = v.Service.Tags
		}
	}
	// the reason why we want this in a batch and not as individual messages is that
	// if we have an atomic batch, we can calculate what nodes have left the cluster
	// passing events one by one, we can't know if someone left or just haven't changed status for a long time

	// publish the current cluster topology onto the event stream
	eventstream.Publish(res)
}

func (p *ConsulProvider) MonitorMemberStatusChanges() {
	go func() {
		for !p.shutdown {
			p.notifyStatuses()
		}
	}()
}

// GetHealthStatus returns an error if the cluster health status has problems
func (p *ConsulProvider) GetHealthStatus() error {
	return p.clusterError
}
