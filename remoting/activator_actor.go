package remoting

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
)

var (
	nameLookup   = make(map[string]actor.Props)
	activatorPid = actor.SpawnNamed(actor.FromProducer(newActivatorActor()), "activator")
)

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

func newActivatorActor() actor.Producer {
	return func() actor.Actor {
		return &activator{}
	}
}

func (*activator) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Println("[CLUSTER] Activator started")
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
