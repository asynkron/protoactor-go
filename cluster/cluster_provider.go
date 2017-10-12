package cluster

type MemberStatus struct {
	MemberID string
	Host     string
	Port     int
	Kinds    []string
	Alive    bool
	Weight   int
}

type ClusterTopologyEvent []*MemberStatus

type ClusterProvider interface {
	RegisterMember(clusterName string, address string, port int, knownKinds []string) error
	MonitorMemberStatusChanges()
	UpdateWeight(weight int) error
	DeregisterMember() error
	Shutdown() error
}
