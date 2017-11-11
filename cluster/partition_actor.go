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
			spawnings:  make(map[string]*spawningProcess),
			kind:       kind,
		}
	}
}

type partitionActor struct {
	partition  map[string]*actor.PID       //actor/grain name to PID
	keyNameMap map[string]string           //actor/grain key to name
	spawnings  map[string]*spawningProcess //spawning actor/grain futures
	kind       string
}

type spawningProcess struct {
	future *actor.Future
	valid  bool
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
	pid := state.partition[msg.Name]
	if pid != nil {
		response := &remote.ActorPidResponse{Pid: pid}
		context.Respond(response)
		return
	}

	sp := state.spawnings[msg.Name]
	if sp != nil {
		context.AwaitFuture(sp.future, func(r interface{}, err error) {
			if !sp.valid {
				context.Respond(remote.ActorPidRespUnavailable)
			} else {
				response, _ := r.(*remote.ActorPidResponse)
				context.Respond(response)
			}
		})
		return
	}

	activator := getActivatorMember(msg.Kind)
	if activator == "" {
		//No activator currently available, return unavailable
		context.Respond(remote.ActorPidRespUnavailable)
		return
	}

	sp = &spawningProcess{
		future: actor.NewFuture(cfg.TimeoutTime * 3),
		valid:  true,
	}
	state.spawnings[msg.Name] = sp
	context.AwaitFuture(sp.future, func(r interface{}, err error) {
		delete(state.spawnings, msg.Name)
		if !sp.valid {
			context.Respond(remote.ActorPidRespUnavailable)
			return
		}
		resp, _ := r.(*remote.ActorPidResponse)
		if resp.StatusCode == remote.ResponseStatusCodeOK.ToInt32() {
			pid = resp.Pid
			state.partition[msg.Name] = pid
			state.keyNameMap[pid.String()] = msg.Name
			context.Watch(pid)
		}
		context.Respond(resp)
	})

	//Spawning
	go state.spawning(msg, activator, 3, sp.future.PID())
}

func (state *partitionActor) spawning(msg *remote.ActorPidRequest, activator string, retryLeft int, fPid *actor.PID) {
	if activator == "" {
		activator = getActivatorMember(msg.Kind)
		if activator == "" {
			//No activator currently available, return unavailable
			fPid.Tell(remote.ActorPidRespUnavailable)
			return
		}
	}

	pidResp, err := remote.SpawnNamed(activator, msg.Name, msg.Kind, cfg.TimeoutTime)
	if err != nil {
		plog.Error("Partition failed to spawn actor", log.String("name", msg.Name), log.String("kind", msg.Kind), log.String("address", activator), log.Error(err))
		if err == actor.ErrTimeout {
			fPid.Tell(remote.ActorPidRespTimeout)
		} else {
			fPid.Tell(remote.ActorPidRespErr)
		}
		return
	}

	if pidResp.StatusCode == remote.ResponseStatusCodeUNAVAILABLE.ToInt32() && retryLeft != 0 {
		retryLeft--
		state.spawning(msg, "", retryLeft, fPid)
		return
	}

	fPid.Tell(pidResp)
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
	//If the left member is self, transfer remaining pids to others
	if msg.Name() == actor.ProcessRegistry.Address {
		for actorID := range state.partition {
			address := getPartitionMember(actorID, state.kind)
			if address != "" {
				state.transferOwnership(actorID, address, context)
			}
		}
		for actorID, sp := range state.spawnings {
			address := getPartitionMember(actorID, state.kind)
			if address != "" {
				sp.valid = false
				sp.future.PID().Tell(remote.ActorPidRespUnavailable)
			}
		}
	}

	for actorID, pid := range state.partition {
		//if the mapped PID is on the address that left, forget it
		if pid.Address == msg.Name() {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, msg.Name())
			delete(state.partition, actorID)
			delete(state.keyNameMap, pid.String())
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
	for actorID, sp := range state.spawnings {
		address := getPartitionMember(actorID, state.kind)
		if address != "" && address != actor.ProcessRegistry.Address {
			sp.valid = false
			sp.future.PID().Tell(remote.ActorPidRespUnavailable)
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
