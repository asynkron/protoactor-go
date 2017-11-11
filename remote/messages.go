package remote

import "github.com/AsynkronIT/protoactor-go/actor"

type StopEndpointManager struct{}

type EndpointTerminatedEvent struct {
	Address string
}

type EndpointConnectedEvent struct {
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

type JsonMessage struct {
	TypeName string
	Json     string
}

var (
	stopMessage interface{} = &actor.Stop{}
)

var (
	ActorPidRespErr         interface{} = &ActorPidResponse{StatusCode: ResponseStatusCodeERROR.ToInt32()}
	ActorPidRespTimeout     interface{} = &ActorPidResponse{StatusCode: ResponseStatusCodeTIMEOUT.ToInt32()}
	ActorPidRespUnavailable interface{} = &ActorPidResponse{StatusCode: ResponseStatusCodeUNAVAILABLE.ToInt32()}
)
