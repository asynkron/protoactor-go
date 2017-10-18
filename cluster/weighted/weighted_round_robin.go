package weighted

import (
	"github.com/AsynkronIT/protoactor-go/cluster"
)

type WeightedRoundRobin struct {
	currIndex  int
	currWeight int
	maxWeight  int
	gcdValue   int
	m          cluster.MemberStrategy
}

func NewWeightedRoundRobin(memberStrategy cluster.MemberStrategy) *WeightedRoundRobin {
	return &WeightedRoundRobin{m: memberStrategy}
}

func (r *WeightedRoundRobin) GetByRoundRobin() string {

	members := r.m.GetAllMembers()
	l := len(members)

	if l == 0 {
		return ""
	}

	if l == 1 {
		return members[0].Address()
	}

	for {
		r.currIndex = (r.currIndex + 1) % l
		if r.currIndex == 0 {
			r.currWeight = r.currWeight - r.gcdValue
			if r.currWeight <= 0 {
				r.currWeight = r.maxWeight
			}
		}
		sv, _ := members[r.currIndex].StatusValue.(*WeightedMemberStatusValue)
		if sv.Weight >= r.currWeight {
			return members[r.currIndex].Address()
		}
	}
}

func (r *WeightedRoundRobin) UpdateRR() {
	r.maxWeight = r.getMaxWeight()
	r.gcdValue = r.getGcd()
}

func (r *WeightedRoundRobin) getMaxWeight() int {
	max := 0
	for _, m := range r.m.GetAllMembers() {
		sv, _ := m.StatusValue.(*WeightedMemberStatusValue)
		if sv.Weight > max {
			max = sv.Weight
		}
	}
	return max
}

func (r *WeightedRoundRobin) getGcd() int {
	members := r.m.GetAllMembers()
	if len(members) == 0 {
		return 0
	}
	ints := make([]int, len(members))
	for i, member := range members {
		sv, _ := member.StatusValue.(*WeightedMemberStatusValue)
		ints[i] = sv.Weight
	}
	return r.ngcd(ints)
}

func (r *WeightedRoundRobin) gcd(a, b int) int {
	if a < b {
		a, b = b, a
	}
	if b == 0 {
		return a
	}
	return r.gcd(b, a%b)
}

func (r *WeightedRoundRobin) ngcd(ints []int) int {
	n := len(ints)
	if n == 1 {
		return ints[0]
	}
	return r.gcd(ints[n-1], r.ngcd(ints[0:n-1]))
}
