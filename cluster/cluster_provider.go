package cluster

//type ClusterState struct {
//	BannedMembers []string `json:"blockedMembers"`
//}

// ClusterProvider integrates the cluster implementation with an underlying membership system.
type ClusterProvider interface {
        StartMember(cluster *Cluster) error
        StartClient(cluster *Cluster) error
        Shutdown(graceful bool) error
        // UpdateClusterState(state ClusterState) error
}
