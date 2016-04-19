package actor

// import (
// 	"github.com/rogeralsing/goactor/interfaces"
// )

// type RouterConfig interface {
// 	Create(PropsValue) actor.ActorRef
// }

// type RoundRobinGroupRouter struct {
// 	routees []actor.ActorRef
// }

// func NewRoundRobinGroupRouter(routees ...actor.ActorRef) RouterConfig {
// 	return &RoundRobinGroupRouter{routees: routees}
// }

// func (config *RoundRobinGroupRouter) Create(props PropsValue) actor.ActorRef {
// 	actorProps := props
// 	actorProps.routerConfig = nil
// 	actor := Spawn(actorProps)
// 	return actor
// }
