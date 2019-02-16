package plugin

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

type plugin interface {
	OnStart(actor.ReceiverContext)
	OnOtherMessage(actor.ReceiverContext, *actor.MessageEnvelope)
}

func Use(plugin plugin) func(next actor.ReceiverFunc) actor.ReceiverFunc {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		fn := func(context actor.ReceiverContext, env *actor.MessageEnvelope) {
			switch env.Message.(type) {
			case *actor.Started:
				plugin.OnStart(context)
			default:
				plugin.OnOtherMessage(context, env)
			}

			next(context, env)
		}
		return fn
	}
}
