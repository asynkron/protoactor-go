package cluster

type MemberStrategy interface {
	GetAllMembers() Members
	AddMember(member *Member)
	RemoveMember(member *Member)
	GetPartition(key string) string
	GetActivator(senderAddress string) string
}

type simpleMemberStrategy struct {
	members Members
	rr      *SimpleRoundRobin
	rdv     *Rendezvous
}

func newDefaultMemberStrategy(cluster *Cluster, kind string) MemberStrategy {
	ms := &simpleMemberStrategy{members: make(Members, 0)}
	ms.rr = NewSimpleRoundRobin(MemberStrategy(ms))
	ms.rdv = NewRendezvous()
	return ms
}

func (m *simpleMemberStrategy) AddMember(member *Member) {
	m.members = append(m.members, member)
	m.rdv.UpdateMembers(m.members)
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
			m.rdv.UpdateMembers(m.members)
			return
		}
	}
}

func (m *simpleMemberStrategy) GetAllMembers() Members {
	return m.members
}

func (m *simpleMemberStrategy) GetPartition(key string) string {
	return m.rdv.GetByIdentity(key)
}

func (m *simpleMemberStrategy) GetActivator(senderAddress string) string {
	return m.rr.GetByRoundRobin()
}
