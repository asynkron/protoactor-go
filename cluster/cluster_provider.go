package cluster

type MemberStatus struct {
	MemberID string
	Host     string
	Port     int
	Kinds    []string
	Alive    bool
}

type ClusterTopologyEvent []*MemberStatus

type ClusterProvider interface {
	RegisterMember(clusterName string, address string, port int, knownKinds []string) error
	MonitorMemberStatusChanges()
	Shutdown() error
}
