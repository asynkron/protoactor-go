package cluster

import "fmt"

type MemberStatusEvent interface {
	MemberStatusEvent()
	GetKinds() []string
}

type MemberMeta struct {
	Host  string
	Port  int
	Kinds []string
}

func (e *MemberMeta) Name() string {
	return fmt.Sprintf("%v:%v", e.Host, e.Port)
}

func (e *MemberMeta) GetKinds() []string {
	return e.Kinds
}

type MemberJoinedEvent struct {
	MemberMeta
}

func (*MemberJoinedEvent) MemberStatusEvent() {}

type MemberRejoinedEvent struct {
	MemberMeta
}

func (*MemberRejoinedEvent) MemberStatusEvent() {}

type MemberLeftEvent struct {
	MemberMeta
}

func (*MemberLeftEvent) MemberStatusEvent() {}

type MemberUnavailableEvent struct {
	MemberMeta
}

func (*MemberUnavailableEvent) MemberStatusEvent() {}

type MemberAvailableEvent struct {
	MemberMeta
}

func (*MemberAvailableEvent) MemberStatusEvent() {}
