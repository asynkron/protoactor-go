package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
)

func newEndpointWatcher(address string) actor.Producer {
	return func() actor.Actor {
		return &endpointWatcher{
			address: address,
		}
	}
}

type endpointWatcher struct {
	address string
	watched map[string]*PIDSet //key is the watching PID string, value is the watched PID
}

func (state *endpointWatcher) initialize() {
	plog.Info("Started EndpointWatcher", log.String("address", state.address))
	state.watched = make(map[string]*PIDSet)
}

func (state *endpointWatcher) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize()

	case *remoteTerminate:
		plog.Info("remoteTerminate handling terminated", log.String("address", state.address))
		var watchee = msg.Watchee

		if watchedPIDs, founded := state.watched[msg.Watcher.Id]; founded {
			//The cache object pointer is different from the temporary object pointer interpreted by the reflector
			if cacheWatchee, deleted := watchedPIDs.Remove(msg.Watchee.Id); deleted {
				watchee = cacheWatchee

				if watchedPIDs.Size() == 0 {
					delete(state.watched, msg.Watcher.Id)
				}
			}
		}
		terminated := &actor.Terminated{
			Who:               watchee,
			AddressTerminated: false,
		}
		ref, ok := actor.ProcessRegistry.GetLocal(msg.Watcher.Id)
		if ok {
			ref.SendSystemMessage(msg.Watcher, terminated)
		}

	case *EndpointTerminatedEvent:
		plog.Info("EndpointWatcher %v  handling terminated", log.String("address", state.address), log.String("address", state.address))
		for id, pids := range state.watched {
			//try to find the watcher ID in the local actor registry
			localWatcher, ok := actor.ProcessRegistry.GetLocal(id)
			if ok {
				for _, watchee := range pids.All() {
					//create a terminated event for the Watched actor

					terminated := &actor.Terminated{
						Who:               watchee,
						AddressTerminated: true,
					}
					watcher := actor.NewLocalPID(id)
					//send the address Terminated event to the Watcher
					localWatcher.SendSystemMessage(watcher, terminated)
				}
				pids.Clean()
			}
		}

		//todo:
		// When switch another behavior,  EndpointWatcher still can not be recovery if remote service is normal,
		// Need to add more events to control behavior change ,used by [cluster.memberlistActor] and [remote.endpoint_manager]
		// At there is a risk of remotewatch at this time ， because the behavior switch is disabled.
		//ctx.SetBehavior(state.Terminated)

	case *remoteWatch:
		var (
			pids    *PIDSet
			founded bool
		)
		if pids, founded = state.watched[msg.Watcher.Id]; !founded {
			pids = &PIDSet{}
			state.watched[msg.Watcher.Id] = pids
		}
		pids.Add(msg.Watchee)

		//recreate the Watch command
		w := &actor.Watch{
			Watcher: msg.Watcher,
		}
		//pass it off to the remote PID
		SendMessage(msg.Watchee, w, nil, -1)

	case *remoteUnwatch:
		//delete the watch entries
		if watchedPIDs, founded := state.watched[msg.Watcher.Id]; founded {
			//cached watchee ptr is not same from reflect from protobuf
			if _, deleted := watchedPIDs.Remove(msg.Watchee.Id); deleted {
				if watchedPIDs.Size() == 0 {
					delete(state.watched, msg.Watcher.Id)
				}
			}
		}
		//recreate the Unwatch command
		uw := &actor.Unwatch{
			Watcher: msg.Watcher,
		}

		//pass it off to the remote PID
		SendMessage(msg.Watchee, uw, nil, -1)

	default:
		plog.Error("EndpointWatcher received unknown message", log.String("address", state.address), log.Message(msg))
	}
}

func (state *endpointWatcher) Terminated(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Restarting:
		ctx.SetBehavior(state.Receive)
	case *remoteWatch:
		//try to find the watcher ID in the local actor registry
		ref, ok := actor.ProcessRegistry.GetLocal(msg.Watcher.Id)
		if ok {

			//create a terminated event for the Watched actor
			terminated := &actor.Terminated{
				Who:               msg.Watchee,
				AddressTerminated: true,
			}
			//send the address Terminated event to the Watcher
			ref.SendSystemMessage(msg.Watcher, terminated)
		}

	case *remoteTerminate, *EndpointTerminatedEvent, *remoteUnwatch:
		// pass

	default:
		plog.Error("EndpointWatcher received unknown message", log.String("address", state.address), log.Message(msg))
	}
}
