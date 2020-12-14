package cluster

type simpleMemberStrategyV2 struct {
	members []*Member // must be sorted
	rr      *SimpleRoundRobin
	rdv     *RendezvousV2
}

func newDefaultMemberStrategyV2(kind string, members []*Member) MemberStrategy {
	ms := &simpleMemberStrategyV2{members: members}
	ms.rdv = NewRendezvousV2(members)
	ms.rr = NewSimpleRoundRobin(MemberStrategy(ms))
	return ms
}

func (m *simpleMemberStrategyV2) GetAllMembers() []*Member {
	return m.members
}

func (m *simpleMemberStrategyV2) GetPartition(key string) string {
	return m.rdv.Get(key)
}

func (m *simpleMemberStrategyV2) GetActivator() string {
	return m.rr.GetByRoundRobin()
}
