package remote

import "github.com/AsynkronIT/protoactor-go/actor"

type EndpointTerminatedEvent struct {
	Address string
}

type remoteWatch struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

type remoteUnwatch struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

type remoteTerminate struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

var (
	stopMessage interface{} = &actor.Stop{}
)
