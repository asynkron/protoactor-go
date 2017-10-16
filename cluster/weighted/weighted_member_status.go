package weighted

import (
	"strconv"

	"github.com/AsynkronIT/protoactor-go/cluster"
)

type WeightedMemberStatusValue struct {
	Weight int
}

func (sv *WeightedMemberStatusValue) IsSame(val cluster.MemberStatusValue) bool {
	if val == nil {
		return false
	}
	if v, ok := val.(*WeightedMemberStatusValue); ok {
		return sv.Weight == v.Weight
	}
	return false
}

type WeightedMemberStatusValueSerializer struct{}

func (s *WeightedMemberStatusValueSerializer) ToValueBytes(val cluster.MemberStatusValue) []byte {
	dVal, _ := val.(*WeightedMemberStatusValue)
	return []byte(strconv.Itoa(dVal.Weight))
}

func (s *WeightedMemberStatusValueSerializer) FromValueBytes(val []byte) cluster.MemberStatusValue {
	weight, _ := strconv.Atoi(string(val))
	return &WeightedMemberStatusValue{Weight: weight}
}
