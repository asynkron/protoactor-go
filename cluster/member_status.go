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
	ToValueBytes(val MemberStatusValue) []byte
	FromValueBytes(val []byte) MemberStatusValue
}

type NilMemberStatusValueSerializer struct{}

func (s *NilMemberStatusValueSerializer) ToValueBytes(val MemberStatusValue) []byte {
	return nil
}

func (s *NilMemberStatusValueSerializer) FromValueBytes(val []byte) MemberStatusValue {
	return nil
}
