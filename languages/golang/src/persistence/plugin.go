package persistence

import (
	"github.com/AsynkronIT/protoactor/languages/golang/src/actor"
	proto "github.com/golang/protobuf/proto"
)

type persistent interface {
	init(provider Provider, context actor.Context)
	PersistReceive(message proto.Message)
	PersistSnapshot(snapshot proto.Message)
	Recovering() bool
	Name() string
}

type Mixin struct {
	eventIndex    int
	providerState ProviderState
	context       actor.Context
	recovering    bool
}

func (mixin *Mixin) Recovering() bool {
	return mixin.recovering
}

func (mixin *Mixin) Name() string {
	name := mixin.context.Self().Id
	return name
}
func (mixin *Mixin) PersistReceive(message proto.Message) {

	mixin.providerState.PersistEvent(mixin.Name(), mixin.eventIndex, message)
	mixin.eventIndex++
	mixin.context.Receive(message)
	if mixin.eventIndex%mixin.providerState.GetSnapshotInterval() == 0 {
		mixin.context.Receive(&RequestSnapshot{})
	}
}

func (mixin *Mixin) PersistSnapshot(snapshot proto.Message) {
	mixin.providerState.PersistSnapshot(mixin.Name(), mixin.eventIndex, snapshot)
}

func (mixin *Mixin) init(provider Provider, context actor.Context) {
	if mixin.providerState == nil {
		mixin.providerState = provider.GetState()
	}

	mixin.eventIndex = 0
	mixin.context = context
	mixin.recovering = true

	mixin.providerState.Restart()
	if snapshot, eventIndex, ok := mixin.providerState.GetSnapshot(mixin.Name()); ok {
		mixin.eventIndex = eventIndex
		context.Receive(snapshot)
	}
	mixin.providerState.GetEvents(mixin.Name(), mixin.eventIndex, func(e interface{}) {
		context.Receive(e)
		mixin.eventIndex++
	})
	mixin.recovering = false
	context.Receive(&ReplayComplete{})
}
