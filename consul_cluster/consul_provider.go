package consul_cluster

import (
	"fmt"
	"log"
	"time"

	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/hashicorp/consul/api"
)

type ConsulProvider struct {
	shutdown    bool
	id          string
	clusterName string
	client      *api.Client
}

func New() (*ConsulProvider, error) {
	client, err := api.NewClient(&api.Config{})
	if err != nil {
		return nil, err
	}
	p := &ConsulProvider{
		client: client,
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
			DeregisterCriticalServiceAfter: "20s",
			TTL: "10s",
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
			log.Println("Refreshing service TTL")
			err := refresh()
			if err != nil {
				log.Println("Failure refreshing service TTL")
			}
			time.Sleep(2 * time.Second)
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

func (p *ConsulProvider) GetStatusChanges() <-chan []*cluster.MemberStatus {
	c := make(chan []*cluster.MemberStatus)
	var index uint64
	healthCheck := func() ([]*api.ServiceEntry, error) {
		res, meta, err := p.client.Health().Service(p.clusterName, "", false, &api.QueryOptions{
			WaitIndex: index,
			WaitTime:  20 * time.Second,
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
			log.Println("Cluster status changed")
			if err != nil {
				log.Printf("Error %v", err)
			} else {
				res := make([]*cluster.MemberStatus, len(statuses))
				for i, v := range statuses {
					ms := &cluster.MemberStatus{
						Address: v.Service.Address,
						Port:    v.Service.Port,
						Kinds:   v.Service.Tags,
						Alive:   v.Checks[0].Status == "passing",
					}
					res[i] = ms
				}
				c <- res
			}
		}
	}()
	return c
}
