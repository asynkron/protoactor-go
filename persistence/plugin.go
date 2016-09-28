package persistence

import (
	proto "github.com/golang/protobuf/proto"
)

type persistent interface {
	init(func(message proto.Message))
	PersistReceive(message proto.Message)
}

type Mixin struct {
	sar func(message proto.Message)
}

func (mixin *Mixin) PersistReceive(message proto.Message) {
	mixin.sar(message)
}

func (mixin *Mixin) init(sar func(message proto.Message)) {
	mixin.sar = sar
}
