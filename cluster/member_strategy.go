package cluster

type MemberStrategy interface {
	GetAllMembers() []*Member
	// AddMember(member *Member)
	// UpdateMember(member *Member)
	// RemoveMember(member *Member)
	GetPartition(key string) string
	GetActivator() string
}

type simpleMemberStrategy struct {
	members []*Member
	rr      *SimpleRoundRobin
	rdv     *Rendezvous
}

func newDefaultMemberStrategy(kind string) MemberStrategy {
	ms := &simpleMemberStrategy{members: make([]*Member, 0)}
	ms.rr = NewSimpleRoundRobin(MemberStrategy(ms))
	ms.rdv = NewRendezvous(MemberStrategy(ms))
	return ms
}

func (m *simpleMemberStrategy) AddMember(member *Member) {
	m.members = append(m.members, member)
	m.rdv.UpdateRdv()
}

func (m *simpleMemberStrategy) UpdateMember(member *Member) {
	for i, mb := range m.members {
		if mb.Address() == member.Address() {
			m.members[i] = member
			return
		}
	}
}

func (m *simpleMemberStrategy) RemoveMember(member *Member) {
	for i, mb := range m.members {
		if mb.Address() == member.Address() {
			m.members = append(m.members[:i], m.members[i+1:]...)
			m.rdv.UpdateRdv()
			return
		}
	}
}

func (m *simpleMemberStrategy) GetAllMembers() []*Member {
	return m.members
}

func (m *simpleMemberStrategy) GetPartition(key string) string {
	return m.rdv.GetByRdv(key)
}

func (m *simpleMemberStrategy) GetActivator() string {
	return m.rr.GetByRoundRobin()
}
