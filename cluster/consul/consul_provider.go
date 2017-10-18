package consul

import (
	"fmt"
	"log"
	"time"

	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/hashicorp/consul/api"
)

type ConsulProvider struct {
	deregistered          bool
	shutdown              bool
	id                    string
	clusterName           string
	address               string
	port                  int
	knownKinds            []string
	index                 uint64 //consul blocking index
	client                *api.Client
	ttl                   time.Duration
	refreshTTL            time.Duration
	deregisterCritical    time.Duration
	blockingWaitTime      time.Duration
	statusValue           cluster.MemberStatusValue
	statusValueSerializer cluster.MemberStatusValueSerializer
}

func New() (*ConsulProvider, error) {
	return NewWithConfig(&api.Config{})
}

func NewWithConfig(consulConfig *api.Config) (*ConsulProvider, error) {
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, err
	}
	p := &ConsulProvider{
		client:             client,
		ttl:                3 * time.Second,
		refreshTTL:         1 * time.Second,
		deregisterCritical: 10 * time.Second,
		blockingWaitTime:   20 * time.Second,
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
	p.statusValueSerializer = serializer

	err := p.registerService()
	if err != nil {
		return err
	}

	err = p.registerMemberID()
	if err != nil {
		return err
	}

	err = p.UpdateMemberStatusValue(statusValue)
	if err != nil {
		return err
	}

	//IMPORTANT: do these ops sync directly after registering.
	//this will ensure that the local node sees its own information upon startup.

	//force our own TTL to be OK
	p.blockingUpdateTTL()
	//force our own existence to be part of the first status update
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
	err = p.deregisterMemberID()
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = p.deleteMemberStatusValue()
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
			p.blockingUpdateTTL()
			time.Sleep(p.refreshTTL)
		}
	}()
}

func (p *ConsulProvider) UpdateMemberStatusValue(statusValue cluster.MemberStatusValue) error {
	p.statusValue = statusValue
	if p.statusValue == nil {
		return nil
	}
	kvKey := fmt.Sprintf("%v/%v:%v/StatusValue", p.clusterName, p.address, p.port)
	_, err := p.client.KV().Put(&api.KVPair{
		Key:   kvKey,
		Value: p.statusValueSerializer.ToValueBytes(p.statusValue), //currently, just a semi unique id for this member
	}, &api.WriteOptions{})
	return err
}

func (p *ConsulProvider) deleteMemberStatusValue() error {
	if p.statusValue == nil {
		return nil
	}
	kvKey := fmt.Sprintf("%v/%v:%v/StatusValue", p.clusterName, p.address, p.port)
	_, err := p.client.KV().Delete(kvKey, &api.WriteOptions{})
	return err
}

func (p *ConsulProvider) blockingUpdateTTL() {
	refresh := func() error {
		err := p.client.Agent().PassTTL("service:"+p.id, "")
		if err != nil {
			return err
		}
		return nil
	}
	//	log.Println("[CLUSTER] [CONSUL] Refreshing service TTL")
	err := refresh()
	if err != nil {
		log.Println("[CLUSTER] [CONSUL] Failure refreshing service TTL")
	}
}

func (p *ConsulProvider) registerService() error {
	s := &api.AgentServiceRegistration{
		ID:      p.id,
		Name:    p.clusterName,
		Tags:    p.knownKinds,
		Address: p.address,
		Port:    p.port,
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: p.deregisterCritical.String(),
			TTL: p.ttl.String(),
		},
	}
	return p.client.Agent().ServiceRegister(s)
}

func (p *ConsulProvider) deregisterService() error {
	return p.client.Agent().ServiceDeregister(p.id)
}

func (p *ConsulProvider) registerMemberID() error {
	//register a unique ID for the current process
	//similar to UID for Akka ActorSystem
	//TODO: Orleans just use an int32 for the unique id called Generation.
	kvKey := fmt.Sprintf("%v/%v:%v/ID", p.clusterName, p.address, p.port)
	_, err := p.client.KV().Put(&api.KVPair{
		Key:   kvKey,
		Value: []byte(time.Now().UTC().Format(time.RFC3339)), //currently, just a semi unique id for this member
	}, &api.WriteOptions{})
	return err
}

func (p *ConsulProvider) deregisterMemberID() error {
	kvKey := fmt.Sprintf("%v/%v:%v/ID", p.clusterName, p.address, p.port)
	_, err := p.client.KV().Delete(kvKey, &api.WriteOptions{})
	return err
}

//call this directly after registering the service
func (p *ConsulProvider) blockingStatusChange() {
	p.notifyStatuses()
}

func (p *ConsulProvider) notifyStatuses() {

	statuses, meta, err := p.client.Health().Service(p.clusterName, "", false, &api.QueryOptions{
		WaitIndex: p.index,
		WaitTime:  p.blockingWaitTime,
	})
	if err != nil {
		log.Printf("Error %v", err)
		return
	}
	p.index = meta.LastIndex

	//fetch additional info per member from the consul KV store
	kvKey := p.clusterName + "/"
	kv, _, err := p.client.KV().List(kvKey, &api.QueryOptions{})
	if err != nil {
		log.Printf("Error %v", err)
		return
	}
	kvMap := make(map[string][]byte)
	for _, v := range kv {
		kvMap[v.Key] = v.Value
	}

	res := make(cluster.ClusterTopologyEvent, len(statuses))
	for i, v := range statuses {
		key := fmt.Sprintf("%v/%v:%v", p.clusterName, v.Service.Address, v.Service.Port)
		memberID := string(kvMap[key+"/ID"])
		memberStatusVal := p.statusValueSerializer.FromValueBytes(kvMap[key+"/StatusValue"])
		ms := &cluster.MemberStatus{
			MemberID:    memberID,
			Host:        v.Service.Address,
			Port:        v.Service.Port,
			Kinds:       v.Service.Tags,
			Alive:       len(v.Checks) > 0 && v.Checks.AggregatedStatus() == api.HealthPassing,
			StatusValue: memberStatusVal,
		}
		res[i] = ms

		//Update Tags for this member
		if memberID == p.id {
			p.knownKinds = v.Service.Tags
		}
	}
	//the reason why we want this in a batch and not as individual messages is that
	//if we have an atomic batch, we can calculate what nodes have left the cluster
	//passing events one by one, we can't know if someone left or just havent changed status for a long time

	//publish the current cluster topology onto the event stream
	eventstream.Publish(res)
}

func (p *ConsulProvider) MonitorMemberStatusChanges() {

	go func() {
		for !p.shutdown {
			p.notifyStatuses()
		}
	}()
}
