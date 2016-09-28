package plugin

import "github.com/AsynkronIT/gam/actor"

func Use(f func(actor.Context)) func(context actor.Context) {
	return func(context actor.Context) {
		switch context.Message().(type) {
		case *actor.Started:
			f(context)
			context.Next()
		default:
			context.Next()
		}
	}
}
