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

// KindUpdater is an optional interface that ClusterProviders can implement
// to support runtime Kind changes. When implemented, the cluster calls
// UpdateKinds after RegisterKind or DeregisterKind is called.
//
// Providers that don't implement this interface still work — they just
// won't announce Kind changes to the cluster until the node restarts.
type KindUpdater interface {
	// UpdateKinds is called when the cluster's registered Kinds change.
	// The provider should re-announce the node with the updated kind list.
	// The kinds slice contains all currently registered Kind names.
	UpdateKinds(kinds []string) error
}
