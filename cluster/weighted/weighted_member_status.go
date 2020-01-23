package weighted

import (
	"strconv"

	"github.com/otherview/protoactor-go/cluster"
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

func (s *WeightedMemberStatusValueSerializer) Serialize(val cluster.MemberStatusValue) string {
	dVal, _ := val.(*WeightedMemberStatusValue)
	return strconv.Itoa(dVal.Weight)
}

func (s *WeightedMemberStatusValueSerializer) Deserialize(val string) cluster.MemberStatusValue {
	weight, _ := strconv.Atoi(val)
	return &WeightedMemberStatusValue{Weight: weight}
}
