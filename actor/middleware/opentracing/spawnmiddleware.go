package opentracing

import (
	"github.com/asynkron/protoactor-go/actor"
	olog "github.com/opentracing/opentracing-go/log"
	"log/slog"
)

func SpawnMiddleware() actor.SpawnMiddleware {
	return func(next actor.SpawnFunc) actor.SpawnFunc {
		return func(actorSystem *actor.ActorSystem, id string, props *actor.Props, parentContext actor.SpawnerContext) (pid *actor.PID, e error) {
			self := parentContext.Self()
			pid, err := next(actorSystem, id, props, parentContext)
			if err != nil {
				actorSystem.Logger().Debug("SPAWN got error trying to spawn", slog.Any("self", self), slog.Any("actor", parentContext.Actor()), slog.Any("error", err))
				return pid, err
			}
			if self != nil {
				span := getActiveSpan(self)
				if span != nil {
					setParentSpan(pid, span)
					span.LogFields(olog.String("SpawnPID", pid.String()))
					actorSystem.Logger().Debug("SPAWN found active span", slog.Any("self", self), slog.Any("actor", parentContext.Actor()), slog.Any("spawned-pid", pid))
				} else {
					actorSystem.Logger().Debug("SPAWN no active span on parent", slog.Any("self", self), slog.Any("actor", parentContext.Actor()), slog.Any("spawned-pid", pid))
				}
			} else {
				actorSystem.Logger().Debug("SPAWN no parent pid", slog.Any("self", self))
			}
			return pid, err
		}
	}
}
