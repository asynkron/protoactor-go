package persistence

import (
	proto "github.com/golang/protobuf/proto"
)

type functions struct {
	persistMessage  func(message proto.Message)
	persistSnapshot func(message proto.Message)
}

type persistent interface {
	init(functions)
	PersistReceive(message proto.Message)
	PersistSnapshot(snapshot proto.Message)
}

type Mixin struct {
	functions
}

func (mixin *Mixin) PersistReceive(message proto.Message) {
	mixin.persistMessage(message)
}

func (mixin *Mixin) PersistSnapshot(snapshot proto.Message) {
	mixin.persistSnapshot(snapshot)
}

func (mixin *Mixin) init(f functions) {
	mixin.functions = f
}
