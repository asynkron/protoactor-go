package remote

import (
	"github.com/asynkron/protoactor-go/actor"
	"log/slog"
)

func newEndpointWatcher(remote *Remote, address string) actor.Producer {
	return func() actor.Actor {
		watcher := &endpointWatcher{
			behavior: actor.NewBehavior(),
			address:  address,
			remote:   remote,
		}
		watcher.behavior.Become(watcher.connected)
		return watcher
	}
}

type endpointWatcher struct {
	behavior actor.Behavior
	address  string
	watched  map[string]*actor.PIDSet // key is the watching PID string, value is the watched PID
	remote   *Remote
}

func (state *endpointWatcher) initialize() {
	state.remote.Logger().Info("Started EndpointWatcher", slog.String("address", state.address))
	state.watched = make(map[string]*actor.PIDSet)
}

func (state *endpointWatcher) Receive(ctx actor.Context) {
	state.behavior.Receive(ctx)
}

func (state *endpointWatcher) connected(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize()

	case *remoteTerminate:
		// delete the watch entries
		if pidSet, ok := state.watched[msg.Watcher.Id]; ok {
			pidSet.Remove(msg.Watchee)
			if pidSet.Len() == 0 {
				delete(state.watched, msg.Watcher.Id)
			}
		}

		terminated := &actor.Terminated{
			Who: msg.Watchee,
			Why: actor.TerminatedReason_Stopped,
		}
		ref, ok := state.remote.actorSystem.ProcessRegistry.GetLocal(msg.Watcher.Id)
		if ok {
			ref.SendSystemMessage(msg.Watcher, terminated)
		}
	case *EndpointConnectedEvent:
		// Already connected, pass
	case *EndpointTerminatedEvent:
		state.remote.Logger().Info("EndpointWatcher handling terminated",
			slog.String("address", state.address), slog.Int("watched", len(state.watched)))

		for id, pidSet := range state.watched {
			// try to find the watcher ExtensionID in the local actor registry
			ref, ok := state.remote.actorSystem.ProcessRegistry.GetLocal(id)
			if ok {
				pidSet.ForEach(func(i int, pid *actor.PID) {
					// create a terminated event for the Watched actor
					terminated := &actor.Terminated{
						Who: pid,
						Why: actor.TerminatedReason_AddressTerminated,
					}

					watcher := state.remote.actorSystem.NewLocalPID(id)
					// send the address Terminated event to the Watcher
					ref.SendSystemMessage(watcher, terminated)
				})
			}
		}

		// Clear watcher's map
		state.watched = make(map[string]*actor.PIDSet)
		state.behavior.Become(state.terminated)
		ctx.Stop(ctx.Self())

	case *remoteWatch:
		// add watchee to watcher's map
		if pidSet, ok := state.watched[msg.Watcher.Id]; ok {
			pidSet.Add(msg.Watchee)
		} else {
			state.watched[msg.Watcher.Id] = actor.NewPIDSet(msg.Watchee)
		}

		// recreate the Watch command
		w := &actor.Watch{
			Watcher: msg.Watcher,
		}

		// pass it off to the remote PID
		state.remote.SendMessage(msg.Watchee, nil, w, nil, -1)

	case *remoteUnwatch:
		// delete the watch entries
		if pidSet, ok := state.watched[msg.Watcher.Id]; ok {
			pidSet.Remove(msg.Watchee)
			if pidSet.Len() == 0 {
				delete(state.watched, msg.Watcher.Id)
			}
		}

		// recreate the Unwatch command
		uw := &actor.Unwatch{
			Watcher: msg.Watcher,
		}

		// pass it off to the remote PID
		state.remote.SendMessage(msg.Watchee, nil, uw, nil, -1)
	case actor.SystemMessage, actor.AutoReceiveMessage:
		// ignore
	default:
		state.remote.Logger().Error("EndpointWatcher received unknown message", slog.String("address", state.address), slog.Any("message", msg))
	}
}

func (state *endpointWatcher) terminated(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *remoteWatch:
		// try to find the watcher ExtensionID in the local actor registry
		ref, ok := state.remote.actorSystem.ProcessRegistry.GetLocal(msg.Watcher.Id)

		if ok {
			// create a terminated event for the Watched actor
			terminated := &actor.Terminated{
				Who: msg.Watchee,
				Why: actor.TerminatedReason_AddressTerminated,
			}
			// send the address Terminated event to the Watcher
			ref.SendSystemMessage(msg.Watcher, terminated)
		}
	case *EndpointConnectedEvent:
		state.remote.Logger().Info("EndpointWatcher handling restart", slog.String("address", state.address))
		state.behavior.Become(state.connected)
	case *remoteTerminate, *EndpointTerminatedEvent, *remoteUnwatch:
		// pass
		state.remote.Logger().Error("EndpointWatcher receive message for already terminated endpoint", slog.String("address", state.address), slog.Any("message", msg))
	case actor.SystemMessage, actor.AutoReceiveMessage:
		// ignore
	default:
		state.remote.Logger().Error("EndpointWatcher received unknown message", slog.String("address", state.address), slog.Any("message", msg))
	}
}
