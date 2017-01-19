package plugin

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

type plugin interface {
	OnStart(actor.Context)
	OnOtherMessage(actor.Context, interface{})
}

func Use(plugin plugin) func(next actor.ReceiveFunc) actor.ReceiveFunc {
	return func(next actor.ReceiveFunc) actor.ReceiveFunc {
		fn := func(context actor.Context) {
			switch msg := context.Message().(type) {
			case *actor.Started:
				plugin.OnStart(context)
			default:
				plugin.OnOtherMessage(context, msg)
			}

			next(context)
		}
		return fn
	}
}
