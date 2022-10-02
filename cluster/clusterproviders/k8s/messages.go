package k8s

// RegisterMember message used to register a new member in k8s
type RegisterMember struct{}

// DeregisterMember Empty struct used to deregister a member from k8s
type DeregisterMember struct{}

// DeregisterMemberResponse sent back from cluster monitor when deregistering completes or fails
type DeregisterMemberResponse struct{}

// StartWatchingCluster message used to start watching a k8s cluster
type StartWatchingCluster struct {
	ClusterName string
}

// StopWatchingCluster message used to stop watching a k8s cluster
type StopWatchingCluster struct{}

// StopWatchingClusterResponse sent back from cluster monitor when stop watching completes
type StopWatchingClusterResponse struct{}
