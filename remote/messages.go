package remote

import "github.com/asynkron/protoactor-go/actor"

// EndpointTerminatedEvent is published when a remote endpoint terminates.
type EndpointTerminatedEvent struct {
	Address string
}

// EndpointConnectedEvent is published when a remote endpoint establishes a connection.
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

type remoteDeliver struct {
	header       actor.ReadonlyMessageHeader
	message      interface{}
	target       *actor.PID
	sender       *actor.PID
	serializerID int32
}

type remoteTerminate struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

// JSONMessage carries a JSON encoded payload and its type name.
type JSONMessage struct {
	TypeName string
	JSON     string
}

var stopMessage interface{} = &actor.Stop{}

var (
	// ActorPidRespErr is returned when spawning an actor results in an error.
	ActorPidRespErr interface{} = &ActorPidResponse{StatusCode: ResponseStatusCodeERROR.ToInt32()}
	// ActorPidRespTimeout is returned when spawning an actor times out.
	ActorPidRespTimeout interface{} = &ActorPidResponse{StatusCode: ResponseStatusCodeTIMEOUT.ToInt32()}
	// ActorPidRespUnavailable is returned when the activator is unavailable.
	ActorPidRespUnavailable interface{} = &ActorPidResponse{StatusCode: ResponseStatusCodeUNAVAILABLE.ToInt32()}
)

type (
	// Ping is message sent by the actor system to probe an actor is started.
	Ping struct{}

	// Pong is response for ping.
	Pong struct{}
)
