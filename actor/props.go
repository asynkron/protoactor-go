package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
)

type PropsValue struct {
	actorProducer       interfaces.ActorProducer
	mailboxProducer     interfaces.MailboxProducer
	supervisionStrategy interfaces.SupervisionStrategy
}

func (props PropsValue) ProduceActor() interfaces.Actor {
	return props.actorProducer()
}

func (props PropsValue) Supervisor() interfaces.SupervisionStrategy {
	return props.supervisionStrategy
}

func (props PropsValue) ProduceMailbox(userInvoke func(interface{}), systemInvoke func(interfaces.SystemMessage)) interfaces.Mailbox {
	return props.mailboxProducer(userInvoke, systemInvoke)
}

func Props(actorProducer interfaces.ActorProducer) PropsValue {
	return PropsValue{
		actorProducer:   actorProducer,
		mailboxProducer: NewQueueMailbox,
	}
}

func (props PropsValue) WithMailbox(mailboxProducer interfaces.MailboxProducer) PropsValue {
	//pass by value, we only modify the copy
	props.mailboxProducer = mailboxProducer
	return props
}

func (props PropsValue) WithSupervisor(supervisor interfaces.SupervisionStrategy) PropsValue {
	//pass by value, we only modify the copy
	props.supervisionStrategy = supervisor
	return props
}
