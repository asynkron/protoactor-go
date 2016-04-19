package actor

// import (
// 	"github.com/rogeralsing/goactor/interfaces"
// )

// type RouterConfig interface {
// 	Create(PropsValue) interfaces.ActorRef
// }

// type RoundRobinGroupRouter struct {
// 	routees []interfaces.ActorRef
// }

// func NewRoundRobinGroupRouter(routees ...interfaces.ActorRef) RouterConfig {
// 	return &RoundRobinGroupRouter{routees: routees}
// }

// func (config *RoundRobinGroupRouter) Create(props PropsValue) interfaces.ActorRef {
// 	actorProps := props
// 	actorProps.routerConfig = nil
// 	actor := Spawn(actorProps)
// 	return actor
// }
