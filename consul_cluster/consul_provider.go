package consul_cluster

import (
	"fmt"
	"log"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/hashicorp/consul/api"
)

type ConsulProvider struct {
	shutdown           bool
	id                 string
	clusterName        string
	client             *api.Client
	ttl                time.Duration
	refreshTTL         time.Duration
	deregisterCritical time.Duration
	blockingWaitTime   time.Duration
}

func New() (*ConsulProvider, error) {
	client, err := api.NewClient(&api.Config{})
	if err != nil {
		return nil, err
	}
	p := &ConsulProvider{
		client:             client,
		ttl:                10 * time.Second,
		refreshTTL:         2 * time.Second,
		deregisterCritical: 60 * time.Second,
		blockingWaitTime:   20 * time.Second,
	}
	return p, nil
}

func (p *ConsulProvider) RegisterMember(clusterName string, address string, port int, knownKinds []string) error {
	p.id = fmt.Sprintf("%v@%v:%v", clusterName, address, port)
	p.clusterName = clusterName
	s := &api.AgentServiceRegistration{
		ID:      p.id,
		Name:    clusterName,
		Tags:    knownKinds,
		Address: address,
		Port:    port,
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: p.deregisterCritical.String(),
			TTL: p.ttl.String(),
		},
	}
	err := p.client.Agent().ServiceRegister(s)

	if err != nil {
		return err
	}

	p.UpdateTTL()
	return nil
}

func (p *ConsulProvider) UpdateTTL() {
	refresh := func() error {
		err := p.client.Agent().PassTTL("service:"+p.id, "")
		if err != nil {
			return err
		}
		return nil
	}

	go func() {
		for !p.shutdown {
			log.Println("[CLUSTER] [CONSUL] Refreshing service TTL")
			err := refresh()
			if err != nil {
				log.Println("[CLUSTER] [CONSUL] Failure refreshing service TTL")
			}
			time.Sleep(p.refreshTTL)
		}
	}()
}

func (p *ConsulProvider) Shutdown() error {
	p.shutdown = true
	err := p.client.Agent().ServiceDeregister(p.id)
	if err != nil {
		return err
	}
	return nil
}

func (p *ConsulProvider) MonitorMemberStatusChanges() {
	var index uint64
	healthCheck := func() ([]*api.ServiceEntry, error) {
		res, meta, err := p.client.Health().Service(p.clusterName, "", false, &api.QueryOptions{
			WaitIndex: index,
			WaitTime:  p.blockingWaitTime,
		})
		if err != nil {
			return nil, err
		}
		index = meta.LastIndex
		return res, nil
	}
	go func() {
		for !p.shutdown {
			statuses, err := healthCheck()
			if err != nil {
				log.Printf("Error %v", err)
			} else {
				res := make(cluster.MemberStatusBatch, len(statuses))
				for i, v := range statuses {
					ms := &cluster.MemberStatus{
						Address: v.Service.Address,
						Port:    v.Service.Port,
						Kinds:   v.Service.Tags,
						Alive:   v.Checks[0].Status == "passing",
					}
					res[i] = ms
				}
				//the reason why we want this in a batch and not as individual messages is that
				//if we have an atomic batch, we can calculate what nodes have left the cluster
				//passing events one by one, we can't know if someone left or just havent changed status for a long time

				//publish the current cluster topology onto the EventStream
				actor.EventStream.Publish(res)
			}
		}
	}()
}
