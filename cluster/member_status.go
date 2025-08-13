package cluster

// MemberStatus represents the availability of a member in the cluster.
type MemberStatus struct {
	Member
	MemberID string // for compatibility
	Alive    bool
}

// Address returns the network address of the member.
func (m *MemberStatus) Address() string {
	return m.Member.Address()
}
