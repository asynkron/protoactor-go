package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	kindPIDMap        map[string]*actor.PID
	partitionKindsSub *eventstream.Subscription
)

func subscribePartitionKindsToEventStream() {
	partitionKindsSub = eventstream.Subscribe(func(m interface{}) {
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

func unsubPartitionKindsToEventStream() {
	eventstream.Unsubscribe(partitionKindsSub)
}

func spawnPartitionActors(kinds []string) {
	kindPIDMap = make(map[string]*actor.PID)
	for _, kind := range kinds {
		kindPID := spawnPartitionActor(kind)
		kindPIDMap[kind] = kindPID
	}
}

func stopPartitionActors() {
	for _, kindPID := range kindPIDMap {
		kindPID.GracefulStop()
	}
}

func spawnPartitionActor(kind string) *actor.PID {
	partitionPid, _ := actor.SpawnNamed(actor.FromProducer(newPartitionActor(kind)), "partition-"+kind)
	return partitionPid
}

func partitionForKind(address, kind string) *actor.PID {
	pid := actor.NewPID(address, "partition-"+kind)
	return pid
}

func newPartitionActor(kind string) actor.Producer {
	return func() actor.Actor {
		return &partitionActor{
			partition:  make(map[string]*actor.PID),
			keyNameMap: make(map[string]string),
			kind:       kind,
		}
	}
}

type partitionActor struct {
	partition  map[string]*actor.PID //actor/grain name to PID
	keyNameMap map[string]string     //actor/grain key to name
	kind       string
}

func (state *partitionActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		plog.Info("Started", log.String("kind", state.kind), log.String("id", context.Self().Id))
	case *remote.ActorPidRequest:
		state.spawn(msg, context)
	case *actor.Terminated:
		state.terminated(msg)
	case *TakeOwnership:
		state.takeOwnership(msg, context)
	case *MemberJoinedEvent:
		state.memberJoined(msg, context)
	case *MemberRejoinedEvent:
		state.memberRejoined(msg)
	case *MemberLeftEvent:
		state.memberLeft(msg, context)
	case *MemberAvailableEvent:
		plog.Info("Member available", log.String("kind", state.kind), log.String("name", msg.Name()))
	case *MemberUnavailableEvent:
		plog.Info("Member unavailable", log.String("kind", state.kind), log.String("name", msg.Name()))
	case actor.SystemMessage, actor.AutoReceiveMessage:
		//ignore
	default:
		plog.Error("Partition got unknown message", log.String("kind", state.kind), log.TypeOf("type", msg), log.Object("msg", msg))
	}
}

func (state *partitionActor) spawn(msg *remote.ActorPidRequest, context actor.Context) {

	//TODO: make this async
	pid := state.partition[msg.Name]
	if pid != nil {
		response := &remote.ActorPidResponse{Pid: pid}
		context.Respond(response)
		return
	}

	activator := getActivatorMember(msg.Kind)
	if activator == "" {
		//No activator currently available, return unavailable
		context.Respond(&remote.ActorPidResponse{StatusCode: remote.ResponseStatusCodeUNAVAILABLE.ToInt32()})
		return
	}

	for retry := 3; retry >= 0; retry-- {
		if activator == "" {
			activator = getActivatorMember(msg.Kind)
			if activator == "" {
				//No activator currently available, return unavailable
				context.Respond(&remote.ActorPidResponse{StatusCode: remote.ResponseStatusCodeUNAVAILABLE.ToInt32()})
				return
			}
		}

		//spawn pid
		resp, err := remote.SpawnNamed(activator, msg.Name, msg.Kind, cfg.TimeoutTime)
		if err != nil {
			plog.Error("Partition failed to spawn actor", log.String("name", msg.Name), log.String("kind", msg.Kind), log.String("address", activator))
			context.Respond(&remote.ActorPidResponse{StatusCode: remote.ResponseStatusCodeERROR.ToInt32()})
			return
		}

		switch remote.ResponseStatusCode(resp.StatusCode) {
		case remote.ResponseStatusCodeOK:
			pid = resp.Pid
			state.partition[msg.Name] = pid
			state.keyNameMap[pid.String()] = msg.Name
			context.Watch(pid)
			context.Respond(resp)
			return
		case remote.ResponseStatusCodeUNAVAILABLE:
			//Retry until failed
			if retry != 0 {
				activator = ""
				continue
			}
			context.Respond(resp)
			return
		default:
			//Forward to requester
			context.Respond(resp)
			return
		}
	}
}

func (state *partitionActor) terminated(msg *actor.Terminated) {
	//one of the actors we manage died, remove it from the lookup
	key := msg.Who.String()
	if name, ok := state.keyNameMap[key]; ok {
		delete(state.partition, name)
		delete(state.keyNameMap, key)
	}
}

func (state *partitionActor) memberRejoined(msg *MemberRejoinedEvent) {
	plog.Info("Member rejoined", log.String("kind", state.kind), log.String("name", msg.Name()))
	for actorID, pid := range state.partition {
		//if the mapped PID is on the address that left, forget it
		if pid.Address == msg.Name() {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, msg.Name())
			delete(state.partition, actorID)
			delete(state.keyNameMap, pid.String())
		}
	}
}

func (state *partitionActor) memberLeft(msg *MemberLeftEvent, context actor.Context) {
	plog.Info("Member left", log.String("kind", state.kind), log.String("name", msg.Name()))
	for actorID, pid := range state.partition {
		//if the mapped PID is on the address that left, forget it
		if pid.Address == msg.Name() {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, msg.Name())
			delete(state.partition, actorID)
			delete(state.keyNameMap, pid.String())
		}
	}

	//If the left member is self, transfer remaining pids to others
	if msg.Name() == actor.ProcessRegistry.Address {
		for actorID := range state.partition {
			address := getPartitionMember(actorID, state.kind)
			if address != "" {
				state.transferOwnership(actorID, address, context)
			}
		}
	}
}

func (state *partitionActor) memberJoined(msg *MemberJoinedEvent, context actor.Context) {
	plog.Info("Member joined", log.String("kind", state.kind), log.String("name", msg.Name()))
	for actorID := range state.partition {
		address := getPartitionMember(actorID, state.kind)
		if address != "" && address != actor.ProcessRegistry.Address {
			state.transferOwnership(actorID, address, context)
		}
	}
}

func (state *partitionActor) transferOwnership(actorID string, address string, context actor.Context) {
	pid := state.partition[actorID]
	owner := partitionForKind(address, state.kind)
	owner.Tell(&TakeOwnership{
		Pid:  pid,
		Name: actorID,
	})
	//we can safely delete this entry as the consisntent hash no longer points to us
	delete(state.partition, actorID)
	delete(state.keyNameMap, pid.String())
	context.Unwatch(pid)
}

func (state *partitionActor) takeOwnership(msg *TakeOwnership, context actor.Context) {
	state.partition[msg.Name] = msg.Pid
	state.keyNameMap[msg.Pid.String()] = msg.Name
	context.Watch(msg.Pid)
}
