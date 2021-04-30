package cluster

type TopologyEvent []*Member

type ClusterState struct {
	BannedMembers []string `json:"bannedMembers"`
}

type ClusterProvider interface {
	StartMember(cluster *Cluster) error
	StartClient(cluster *Cluster) error
	Shutdown(graceful bool) error
	UpdateClusterState(state ClusterState) error
}
