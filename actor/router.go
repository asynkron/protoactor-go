package actor

type RouterConfig interface {
	Create(PropsValue) ActorRef
}

type RoundRobinGroupRouter struct {
	routees []ActorRef
}

func NewRoundRobinGroupRouter(routees ...ActorRef) RouterConfig {
    return &RoundRobinGroupRouter { routees: routees}
}

func (config *RoundRobinGroupRouter) Create(props PropsValue) ActorRef {
	props.routerConfig = nil
	actor := ActorOf(props)
	return actor
}
