package cluster

import "strconv"

type MemberStatus struct {
	MemberID    string
	Host        string
	Port        int
	Kinds       []string
	Alive       bool
	StatusValue MemberStatusValue
}

func (m *MemberStatus) Address() string {
	return m.Host + ":" + strconv.Itoa(m.Port)
}

type MemberStatusValue interface {
	IsSame(val MemberStatusValue) bool
}

type MemberStatusValueSerializer interface {
	Serialize(val MemberStatusValue) string
	Deserialize(val string) MemberStatusValue
}

type NilMemberStatusValueSerializer struct{}

func (s *NilMemberStatusValueSerializer) Serialize(val MemberStatusValue) string {
	return ""
}

func (s *NilMemberStatusValueSerializer) Deserialize(val string) MemberStatusValue {
	return nil
}
