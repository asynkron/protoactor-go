package k8s

// Message used to register a new member in k8s
type RegisterMember struct{}

// Empty struct used to deregister a member from k8s
type DeregisterMember struct{}

// Message used to start watching a k8s cluster
type StartWatchingCluster struct {
	ClusterName string
}
