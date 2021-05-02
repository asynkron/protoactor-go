package cluster

type Members struct {
	members []*Member
	lookup  map[string]*Member
}

func NewMembers(members []*Member) *Members {
	SortMembers(members)
	lookup := MembersToMap(members)
	ms := &Members{
		members: members,
		lookup:  lookup,
	}
	return ms
}

func (ms *Members) Members() []*Member {
	return ms.members
}

func (ms *Members) ContainsId(id string) bool {
	_, ok := ms.lookup[id]
	return ok
}

func (ms *Members) GetMemberById(id string) *Member {
	member, _ := ms.lookup[id]
	return member
}
