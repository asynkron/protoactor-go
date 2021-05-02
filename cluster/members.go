package cluster

type MemberSet struct {
	topologyHash uint64
	members      []*Member
	lookup       map[string]*Member
}

var emptyMemberSet = NewMemberSet(make([]*Member, 0))

func NewMemberSet(members []*Member) *MemberSet {
	SortMembers(members)
	lookup := MembersToMap(members)
	ms := &MemberSet{
		topologyHash: TopologyHash(members),
		members:      members,
		lookup:       lookup,
	}
	return ms
}

func (ms *MemberSet) Len() int {
	return len(ms.members)
}

func (ms *MemberSet) TopologyHash() uint64 {
	return ms.topologyHash
}

func (ms *MemberSet) Members() []*Member {
	return ms.members
}

func (ms *MemberSet) ContainsId(id string) bool {
	_, ok := ms.lookup[id]
	return ok
}

func (ms *MemberSet) GetMemberById(id string) *Member {
	member, _ := ms.lookup[id]
	return member
}

func (ms *MemberSet) Except(other *MemberSet) *MemberSet {

	res := make([]*Member, 0)
	for _, m := range ms.members {
		if other.ContainsId(m.Id) {
			continue
		}

		res = append(res, m)
	}

	return NewMemberSet(res)
}

func (ms *MemberSet) Union(other *MemberSet) *MemberSet {
	mapp := make(map[string]*Member, 0)
	for _, m := range ms.members {
		mapp[m.Id] = m
	}
	for _, m := range other.members {
		mapp[m.Id] = m
	}
	res := make([]*Member, 0)
	for _, m := range mapp {
		res = append(res, m)
	}

	return NewMemberSet(res)
}

func (ms *MemberSet) Equals(other *MemberSet) bool {
	return ms.topologyHash == other.topologyHash
}
