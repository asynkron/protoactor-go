package cluster

type MemberStatus struct {
	Address string
	Port    int
	Kinds   []string
	Alive   bool
}
type ClusterProvider interface {
	RegisterNode(clusterName string, address string, port int, knownKinds []string) error
	MemberStatusChanges() <-chan []*MemberStatus
	Shutdown() error
}
