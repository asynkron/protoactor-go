package remote

import (
	"errors"
	"log"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

var (
	nameLookup   = make(map[string]actor.Props)
	activatorPid *actor.PID
)

func spawnActivatorActor() {
	activatorPid, _ = actor.SpawnNamed(actor.FromProducer(newActivatorActor()), "activator")
}

//Register a known actor props by name
func Register(kind string, props *actor.Props) {
	nameLookup[kind] = *props
}

func GetKnownKinds() []string {
	keys := make([]string, 0, len(nameLookup))
	for k := range nameLookup {
		keys = append(keys, k)
	}
	return keys
}

type activator struct {
}

func ActivatorForAddress(address string) *actor.PID {
	pid := actor.NewPID(address, "activator")
	return pid
}

func SpawnFuture(address string, name string, kind string, timeout time.Duration) *actor.Future {
	activator := ActivatorForAddress(address)
	f := activator.RequestFuture(&ActorPidRequest{
		Name: name,
		Kind: kind,
	}, timeout)
	return f
}

func Spawn(address string, kind string, timeout time.Duration) (*actor.PID, error) {
	return SpawnNamed(address, "", kind, timeout)
}

func SpawnNamed(address string, name string, kind string, timeout time.Duration) (*actor.PID, error) {
	activator := ActivatorForAddress(address)
	res, err := activator.RequestFuture(&ActorPidRequest{
		Name: name,
		Kind: kind,
	}, timeout).Result()
	if err != nil {
		return nil, errors.New("[REMOTING] Remote activating timed out")
	}
	switch msg := res.(type) {
	case *ActorPidResponse:
		return msg.Pid, nil
	default:
		return nil, errors.New("[REMOTING] Unknown response when remote activating")
	}
}

func newActivatorActor() actor.Producer {
	return func() actor.Actor {
		return &activator{}
	}
}

func (*activator) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Println("[REMOTING] Started Activator")
	case *ActorPidRequest:
		props := nameLookup[msg.Kind]
		name := msg.Name

		//unnamed actor, assign auto ID
		if name == "" {
			name = actor.ProcessRegistry.NextId()
		}

		pid, _ := actor.SpawnNamed(&props, "Remote$"+msg.Name)
		response := &ActorPidResponse{
			Pid: pid,
		}
		context.Respond(response)
	default:
		log.Printf("[CLUSTER] Activator got unknown message %+v", msg)
	}
}
