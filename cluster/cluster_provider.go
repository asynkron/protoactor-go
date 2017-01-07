package cluster

type MemberStatus struct {
	Address string
	Port    int
	Kinds   []string
	Alive   bool
}

type MemberStatusBatch []*MemberStatus

type ClusterProvider interface {
	RegisterNode(clusterName string, address string, port int, knownKinds []string) error
	MonitorMemberStatusChanges()
	Shutdown() error
}
