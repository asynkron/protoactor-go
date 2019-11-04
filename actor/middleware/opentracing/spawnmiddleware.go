package opentracing

import (
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/log"
	olog "github.com/opentracing/opentracing-go/log"
)

func SpawnMiddleware() actor.SpawnMiddleware {
	return func(next actor.SpawnFunc) actor.SpawnFunc {
		return func(id string, props *actor.Props, parentContext actor.SpawnerContext) (pid *actor.PID, e error) {
			self := parentContext.Self()
			pid, err := next(id, props, parentContext)
			if err != nil {
				logger.Debug("SPAWN got error trying to spawn", log.Stringer("PID", self), log.TypeOf("ActorType", parentContext.Actor()), log.Error(err))
				return pid, err
			}
			if self != nil {
				span := getActiveSpan(self)
				if span != nil {
					setParentSpan(pid, span)
					span.LogFields(olog.String("SpawnPID", pid.String()))
					logger.Debug("SPAWN found active span", log.Stringer("PID", self), log.TypeOf("ActorType", parentContext.Actor()), log.Stringer("SpawnedPID", pid))
				} else {
					logger.Debug("SPAWN no active span on parent", log.Stringer("PID", self), log.TypeOf("ActorType", parentContext.Actor()), log.Stringer("SpawnedPID", pid))
				}
			} else {
				logger.Debug("SPAWN no parent pid", log.Stringer("SpawnedPID", pid))
			}
			return pid, err
		}
	}
}
