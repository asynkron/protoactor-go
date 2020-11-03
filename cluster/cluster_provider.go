package cluster

type ClusterTopologyEvent []*MemberStatus

type ClusterProvider interface {
	RegisterMember(cluster *Cluster, clusterName string, address string, port int, knownKinds []string,
		statusValue MemberStatusValue, serializer MemberStatusValueSerializer) error
	MonitorMemberStatusChanges()
	UpdateMemberStatusValue(statusValue MemberStatusValue) error
	DeregisterMember() error
	Shutdown() error
}
