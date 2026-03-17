package cluster

// ActivatorStrategy selects which cluster member should host a new grain.
// This is distinct from the existing MemberStrategy interface which handles
// partition routing. ActivatorStrategy is specifically for placement decisions
// when spawning new grains via storage-backed identity providers.
type ActivatorStrategy interface {
	// GetActivator returns the member that should spawn the given identity.
	// senderAddress is the address of the node that received the original
	// request. Returns nil if no suitable member is available.
	GetActivator(ci *ClusterIdentity, senderAddress string) *Member

	// AddMember is called when a member joins the cluster.
	AddMember(member *Member)

	// RemoveMember is called when a member leaves the cluster.
	RemoveMember(member *Member)

	// Close releases resources (e.g., unsubscribes from EventStream).
	Close()
}
