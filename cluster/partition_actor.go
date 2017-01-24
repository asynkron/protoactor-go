package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	kindPIDMap map[string]*actor.PID
)

func subscribePartitionKindsToEventStream() {
	eventstream.Subscribe(func(m interface{}) {
		if mse, ok := m.(MemberStatusEvent); ok {
			for _, k := range mse.GetKinds() {
				kindPID := kindPIDMap[k]
				if kindPID != nil {
					kindPID.Tell(m)
				}
			}
		}
	})
}

func spawnPartitionActor(kind string) *actor.PID {
	partitionPid, _ := actor.SpawnNamed(actor.FromProducer(newPartitionActor(kind)), "#partition-"+kind)
	return partitionPid
}

func partitionForKind(address, kind string) *actor.PID {
	pid := actor.NewPID(address, "#partition-"+kind)
	return pid
}

func newPartitionActor(kind string) actor.Producer {
	return func() actor.Actor {
		return &partitionActor{
			partition: make(map[string]*actor.PID),
			kind:      kind,
		}
	}
}

type partitionActor struct {
	partition map[string]*actor.PID //actor/grain name to PID
	kind      string
}

func (state *partitionActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		plog.Info("Started", log.String("id", context.Self().Id))
	case *remote.ActorPidRequest:
		state.spawn(msg, context)
	case *MemberJoinedEvent:
		state.memberJoined(msg)
	case *MemberRejoinedEvent:
		state.memberRejoined(msg)
	case *MemberLeftEvent:
		state.memberLeft(msg)
	case *MemberAvailableEvent:
		plog.Info("Node available", log.String("name", msg.Name()))
	case *MemberUnavailableEvent:
		plog.Info("Node unavailable", log.String("name", msg.Name()))
	case *TakeOwnership:

		state.takeOwnership(msg)
	default:
		plog.Error("Partition got unknown message", log.Object("msg", msg))
	}
}

func (state *partitionActor) spawn(msg *remote.ActorPidRequest, context actor.Context) {

	//TODO: make this async
	pid := state.partition[msg.Name]
	if pid == nil {
		//get a random node
		random := getRandomActivator(msg.Kind)
		var err error
		pid, err = remote.SpawnNamed(random, msg.Name, msg.Kind, 5*time.Second)
		if err != nil {
			plog.Error("Partition failed to spawn actor", log.String("name", msg.Name), log.String("kind", msg.Kind), log.String("address", random))
			return
		}
		state.partition[msg.Name] = pid
	}
	response := &remote.ActorPidResponse{
		Pid: pid,
	}
	context.Respond(response)
}

func (state *partitionActor) memberRejoined(msg *MemberRejoinedEvent) {
	plog.Info("Node rejoined", log.String("name", msg.Name()))
	for actorID, pid := range state.partition {
		//if the mapped PID is on the address that left, forget it
		if pid.Address == msg.Name() {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, msg.Name())
			delete(state.partition, actorID)
		}
	}
}

func (state *partitionActor) memberLeft(msg *MemberLeftEvent) {
	plog.Info("Node left", log.String("name", msg.Name()))
	for actorID, pid := range state.partition {
		//if the mapped PID is on the address that left, forget it
		if pid.Address == msg.Name() {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, msg.Name())
			delete(state.partition, actorID)
		}
	}
}

func (state *partitionActor) memberJoined(msg *MemberJoinedEvent) {
	plog.Info("Node joined", log.String("name", msg.Name()))
	for actorID := range state.partition {
		address := getNode(actorID, state.kind)
		if address != actor.ProcessRegistry.Address {
			state.transferOwnership(actorID, address)
		}
	}
}

func (state *partitionActor) transferOwnership(actorID string, address string) {
	pid := state.partition[actorID]
	owner := partitionForKind(address, state.kind)
	owner.Tell(&TakeOwnership{
		Pid:  pid,
		Name: actorID,
	})
	//we can safely delete this entry as the consisntent hash no longer points to us
	delete(state.partition, actorID)
}

func (state *partitionActor) takeOwnership(msg *TakeOwnership) {
	state.partition[msg.Name] = msg.Pid
}
