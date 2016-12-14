package remoting

import (
	"errors"
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
)

var (
	nameLookup   = make(map[string]actor.Props)
	activatorPid *actor.PID
)

func spawnActivatorActor() {
	activatorPid = actor.SpawnNamed(actor.FromProducer(newActivatorActor()), "activator")
}

//Register a known actor props by name
func Register(kind string, props actor.Props) {
	nameLookup[kind] = props
}

type activator struct {
}

func ActivatorForHost(host string) *actor.PID {
	pid := actor.NewPID(host, "activator")
	return pid
}

func Spawn(host string, name string, kind string, timeout time.Duration) (*actor.PID, error) {
	activator := ActivatorForHost(host)
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
		pid := actor.SpawnNamed(props, "Remote$"+msg.Name)
		response := &ActorPidResponse{
			Pid: pid,
		}
		context.Respond(response)
	default:
		log.Printf("[CLUSTER] Activator got unknown message %+v", msg)
	}
}
