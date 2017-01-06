package consul_cluster

import "github.com/AsynkronIT/protoactor-go/cluster"

type ConsulProvider struct {
}

func (p *ConsulProvider) RegisterNode(knownKinds []string) error {
	return nil
}
func (p *ConsulProvider) GetStatusChanges() <-chan cluster.MemberStatus {
	return nil
}
