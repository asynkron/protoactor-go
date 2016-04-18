package actor

type ActorProducer func() Actor

type PropsValue struct {
	producer     ActorProducer
	routerConfig RouterConfig
}

func Props(producer ActorProducer) PropsValue {
	return PropsValue{producer: producer}
}

func (props PropsValue) WithRouter(routerConfig RouterConfig) PropsValue {
    //pass by value, we only modify the copy
    props.routerConfig = routerConfig
	return props
}
