package cluster

type MemberStatus struct {
	Member
	MemberID string // for compatibility
	Alive    bool
}

func (m *MemberStatus) Address() string {
	return m.Member.Address()
}
