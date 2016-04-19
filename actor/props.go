package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
)

type PropsValue struct {
	actorProducer       interfaces.ActorProducer
	mailbox             interfaces.Mailbox
	supervisionStrategy interfaces.SupervisionStrategy
}

func (props PropsValue) ProduceActor() interfaces.Actor {
	return props.actorProducer()
}

func (props PropsValue) Supervisor() interfaces.SupervisionStrategy {
	return props.supervisionStrategy
}

func (props PropsValue) Mailbox() interfaces.Mailbox {
	if props.mailbox == nil {
		return NewUnboundedMailbox()
	}
	return props.mailbox
}

func Props(actorProducer interfaces.ActorProducer) PropsValue {
	return PropsValue{
		actorProducer: actorProducer,
		mailbox:       nil,
	}
}

func (props PropsValue) WithMailbox(mailbox interfaces.Mailbox) PropsValue {
	//pass by value, we only modify the copy
	props.mailbox = mailbox
	return props
}

func (props PropsValue) WithSupervisor(supervisor interfaces.SupervisionStrategy) PropsValue {
	//pass by value, we only modify the copy
	props.supervisionStrategy = supervisor
	return props
}
