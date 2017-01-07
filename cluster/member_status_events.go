package cluster

import "fmt"

type MemberStatusEvent interface {
	MemberStatusEvent()
	GetKinds() []string
}

type MemberEvent struct {
	Address string
	Port    int
	Kinds   []string
}

func (e *MemberEvent) Name() string {
	return fmt.Sprintf("%v:%v", e.Address, e.Port)
}

func (e *MemberEvent) GetKinds() []string {
	return e.Kinds
}

func (*MemberEvent) MemberStatusEvent() {}

type MemberJoinedEvent struct {
	MemberEvent
}

type MemberLeftEvent struct {
	MemberEvent
}

type MemberUnavailableEvent struct {
	MemberEvent
}

type MemberAvailableEvent struct {
	MemberEvent
}
