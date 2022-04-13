package cluster

import "github.com/asynkron/gofun/set"

type MemberSet struct {
	topologyHash uint64
	members      Members
	lookup       map[string]*Member
}

var emptyMemberSet = NewMemberSet(make(Members, 0))

func NewMemberSet(members Members) *MemberSet {
	members = CopySortMembers(members)
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

func (ms *MemberSet) Members() Members {
	return ms.members
}

func (ms *MemberSet) ContainsID(id string) bool {
	_, ok := ms.lookup[id]

	return ok
}

func (ms *MemberSet) GetMemberById(id string) *Member {
	member, _ := ms.lookup[id]

	return member
}

func (ms *MemberSet) Except(other *MemberSet) *MemberSet {
	res := make(Members, 0)

	for _, m := range ms.members {
		if other.ContainsID(m.Id) {
			continue
		}

		res = append(res, m)
	}

	return NewMemberSet(res)
}

func (ms *MemberSet) ExceptIds(ids []string) *MemberSet {
	other := set.New(ids...)
	res := make(Members, 0)

	for _, m := range ms.members {
		if other.Contains(m.Id) {
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

	res := make(Members, 0)

	for _, m := range mapp {
		res = append(res, m)
	}

	return NewMemberSet(res)
}

func (ms *MemberSet) Equals(other *MemberSet) bool {
	return ms.topologyHash == other.topologyHash
}
