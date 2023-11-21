package remote

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// Register a known actor props by name
func (r *Remote) Register(kind string, props *actor.Props) {
	r.kinds[kind] = props
}

// GetKnownKinds returns a slice of known actor "Kinds"
func (r *Remote) GetKnownKinds() []string {
	keys := make([]string, 0, len(r.kinds))
	for k := range r.kinds {
		keys = append(keys, k)
	}
	return keys
}

type activator struct {
	remote *Remote
}

// ErrActivatorUnavailable : this error will not panic the Activator.
// It simply tells Partition this Activator is not available
// Partition will then find next available Activator to spawn
var ErrActivatorUnavailable = &ActivatorError{ResponseStatusCodeUNAVAILABLE.ToInt32(), true}

type ActivatorError struct {
	Code       int32
	DoNotPanic bool
}

func (e *ActivatorError) Error() string {
	return fmt.Sprint(e.Code)
}

// ActivatorForAddress returns a PID for the activator at the given address
func (r *Remote) ActivatorForAddress(address string) *actor.PID {
	pid := actor.NewPID(address, "activator")
	return pid
}

// SpawnFuture spawns a remote actor and returns a Future that completes once the actor is started
func (r *Remote) SpawnFuture(address, name, kind string, timeout time.Duration) *actor.Future {
	activator := r.ActivatorForAddress(address)
	f := r.actorSystem.Root.RequestFuture(activator, &ActorPidRequest{
		Name: name,
		Kind: kind,
	}, timeout)
	return f
}

// Spawn spawns a remote actor of a given type at a given address
func (r *Remote) Spawn(address, kind string, timeout time.Duration) (*ActorPidResponse, error) {
	return r.SpawnNamed(address, "", kind, timeout)
}

// SpawnNamed spawns a named remote actor of a given type at a given address
func (r *Remote) SpawnNamed(address, name, kind string, timeout time.Duration) (*ActorPidResponse, error) {
	res, err := r.SpawnFuture(address, name, kind, timeout).Result()
	if err != nil {
		return nil, err
	}
	switch msg := res.(type) {
	case *ActorPidResponse:
		return msg, nil
	default:
		return nil, errors.New("remote: Unknown response when remote activating")
	}
}

func newActivatorActor(remote *Remote) actor.Producer {
	return func() actor.Actor {
		return &activator{
			remote: remote,
		}
	}
}

func (a *activator) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		context.Logger().Info("Started Activator")
	case *Ping:
		context.Respond(&Pong{})
	case *ActorPidRequest:
		props, exist := a.remote.kinds[msg.Kind]

		// if props not exist, return error and panic
		if !exist {
			response := &ActorPidResponse{
				StatusCode: ResponseStatusCodeERROR.ToInt32(),
			}
			context.Respond(response)
			panic(fmt.Errorf("no Props found for kind %s", msg.Kind))
		}

		name := msg.Name

		// unnamed actor, assign auto ExtensionID
		if name == "" {
			name = context.ActorSystem().ProcessRegistry.NextId()
		}

		pid, err := context.SpawnNamed(props, "Remote$"+name)

		if err == nil {
			response := &ActorPidResponse{Pid: pid}
			context.Respond(response)
		} else if err == actor.ErrNameExists {
			response := &ActorPidResponse{
				Pid:        pid,
				StatusCode: ResponseStatusCodePROCESSNAMEALREADYEXIST.ToInt32(),
			}
			context.Respond(response)
		} else if aErr, ok := err.(*ActivatorError); ok {
			response := &ActorPidResponse{
				StatusCode: aErr.Code,
			}
			context.Respond(response)
			if !aErr.DoNotPanic {
				panic(err)
			}
		} else {
			response := &ActorPidResponse{
				StatusCode: ResponseStatusCodeERROR.ToInt32(),
			}
			context.Respond(response)
			panic(err)
		}
	case actor.SystemMessage, actor.AutoReceiveMessage:
		// ignore
	default:
		context.Logger().Error("Activator received unknown message", slog.Any("message", msg))
	}
}
