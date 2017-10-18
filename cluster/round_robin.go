package cluster

type SimpleRoundRobin struct {
	val int
	m   MemberStrategy
}

func NewSimpleRoundRobin(memberStrategy MemberStrategy) *SimpleRoundRobin {
	return &SimpleRoundRobin{m: memberStrategy}
}

func (r *SimpleRoundRobin) GetByRoundRobin() string {
	members := r.m.GetAllMembers()
	l := len(members)
	if l == 0 {
		return ""
	}
	if l == 1 {
		return members[0].Address()
	}
	r.val++
	return members[r.val%l].Address()
}
