package cluster

import "sync/atomic"

type SimpleRoundRobin struct {
	val int32
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
	nv := atomic.AddInt32(&r.val, 1)
	return members[int(nv)%l].Address()
}
