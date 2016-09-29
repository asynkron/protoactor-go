package persistence

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
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
	eventIndex int
	provider   Provider
	context    actor.Context
	recovering bool
}

func (mixin *Mixin) Recovering() bool {
	return mixin.recovering
}

func (mixin *Mixin) Name() string {
	name := mixin.context.Self().Id
	return name
}
func (mixin *Mixin) PersistReceive(message proto.Message) {

	mixin.provider.PersistEvent(mixin.Name(), mixin.eventIndex, message)
	mixin.eventIndex++
	mixin.context.Receive(message)
	if mixin.eventIndex%mixin.provider.GetSnapshotInterval() == 0 {
		log.Println("Requesting snapshot")
		mixin.context.Receive(&RequestSnapshot{})
	}
}

func (mixin *Mixin) PersistSnapshot(snapshot proto.Message) {
	log.Println("Persisting snapshot")
	mixin.provider.PersistSnapshot(mixin.Name(), mixin.eventIndex, snapshot)
}

func (mixin *Mixin) init(provider Provider, context actor.Context) {
	mixin.eventIndex = 0
	mixin.context = context
	mixin.provider = provider
	mixin.recovering = true

	if snapshot, eventIndex, ok := provider.GetSnapshot(mixin.Name()); ok {
		mixin.eventIndex = eventIndex
		log.Println("Sending Snapshot")
		context.Receive(snapshot)
	}
	log.Println("Sending Events")
	mixin.provider.GetEvents(mixin.Name(), mixin.eventIndex, func(e interface{}) {
		context.Receive(e)
		mixin.eventIndex++
	})
	mixin.recovering = false
	context.Receive(&ReplayComplete{})
}
