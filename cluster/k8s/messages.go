package k8s

// RegisterMember message used to register a new member in k8s
type RegisterMember struct{}

// DeregisterMember Empty struct used to deregister a member from k8s
type DeregisterMember struct{}

// StartWatchingCluster message used to start watching a k8s cluster
type StartWatchingCluster struct {
	ClusterName string
}
