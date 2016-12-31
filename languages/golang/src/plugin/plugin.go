package plugin

import "github.com/AsynkronIT/protoactor/languages/golang/src/actor"

type plugin interface {
	OnStart(actor.Context)
	OnOtherMessage(actor.Context, interface{})
}

func Use(plugin plugin) func(context actor.Context) {
	return func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *actor.Started:
			plugin.OnStart(context)
			context.Next()
		default:
			plugin.OnOtherMessage(context, msg)
			context.Next()
		}
	}
}
