package cluster

type MemberStatus struct {
	MemberID string
	Address  string
	Port     int
	Kinds    []string
	Alive    bool
}

type MemberStatusBatch []*MemberStatus

type ClusterProvider interface {
	RegisterMember(clusterName string, address string, port int, knownKinds []string) error
	MonitorMemberStatusChanges()
	Shutdown() error
}
