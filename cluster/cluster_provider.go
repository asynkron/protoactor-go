package cluster

type MemberStatus struct {
	Address    string
	KnownKinds []string
	Alive      bool
}
type ClusterProvider interface {
	RegisterNode(knownKinds []string) error
	MemberStatusChanges() <-chan MemberStatus
}
