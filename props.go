package gam

type Properties interface {
	ProduceActor() Actor
	Mailbox() Mailbox
	Supervisor() SupervisionStrategy
}

type PropsValue struct {
	actorProducer       ActorProducer
	mailbox             Mailbox
	supervisionStrategy SupervisionStrategy
}

func (props PropsValue) ProduceActor() Actor {
	return props.actorProducer()
}

func (props PropsValue) Supervisor() SupervisionStrategy {
	return props.supervisionStrategy
}

func (props PropsValue) Mailbox() Mailbox {
	if props.mailbox == nil {
		return NewUnboundedMailbox()
	}
	return props.mailbox
}

func Props(actorProducer ActorProducer) PropsValue {
	return PropsValue{
		actorProducer: actorProducer,
		mailbox:       nil,
	}
}

func (props PropsValue) WithMailbox(mailbox Mailbox) PropsValue {
	//pass by value, we only modify the copy
	props.mailbox = mailbox
	return props
}

func (props PropsValue) WithSupervisor(supervisor SupervisionStrategy) PropsValue {
	//pass by value, we only modify the copy
	props.supervisionStrategy = supervisor
	return props
}
