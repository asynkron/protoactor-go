package cluster

//type ClusterState struct {
//	BannedMembers []string `json:"blockedMembers"`
//}

type ClusterProvider interface {
	StartMember(cluster *Cluster) error
	StartClient(cluster *Cluster) error
	Shutdown(graceful bool) error
	// UpdateClusterState(state ClusterState) error
}
