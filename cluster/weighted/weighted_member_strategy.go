package weighted

import "github.com/AsynkronIT/protoactor-go/cluster"

type WeightedMemberStrategy struct {
	members []*cluster.MemberStatus
	wrr     *WeightedRoundRobin
	rdv     *cluster.Rendezvous
}

func NewWeightedMemberStrategy(kind string) cluster.MemberStrategy {
	ms := &WeightedMemberStrategy{members: make([]*cluster.MemberStatus, 0)}
	ms.wrr = NewWeightedRoundRobin(cluster.MemberStrategy(ms))
	ms.rdv = cluster.NewRendezvous(cluster.MemberStrategy(ms))
	return ms
}

func (m *WeightedMemberStrategy) AddMember(member *cluster.MemberStatus) {
	m.members = append(m.members, member)
	m.wrr.UpdateRR()
	m.rdv.UpdateRdv()
}

func (m *WeightedMemberStrategy) UpdateMember(member *cluster.MemberStatus) {
	for i, mb := range m.members {
		if mb.Address() == member.Address() {
			m.members[i] = member
			m.wrr.UpdateRR()
			return
		}
	}
}

func (m *WeightedMemberStrategy) RemoveMember(member *cluster.MemberStatus) {
	for i, mb := range m.members {
		if mb.Address() == member.Address() {
			copy(m.members[i:], m.members[i+1:])
			m.members[len(m.members)-1] = nil
			m.members = m.members[:len(m.members)-1]

			m.wrr.UpdateRR()
			m.rdv.UpdateRdv()
			return
		}
	}
}

func (m *WeightedMemberStrategy) GetAllMembers() []*cluster.MemberStatus {
	return m.members
}

func (m *WeightedMemberStrategy) GetPartition(key string) string {
	return m.rdv.GetByRdv(key)
}

func (m *WeightedMemberStrategy) GetActivator() string {
	return m.wrr.GetByRoundRobin()
}
