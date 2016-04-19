package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
	"github.com/rogeralsing/goactor/mailbox"
)

type PropsValue struct {
	actorProducer   interfaces.ActorProducer
	mailboxProducer interfaces.MailboxProducer
}

func (props PropsValue) ProduceActor() interfaces.Actor {
	return props.actorProducer()
}

func (props PropsValue) ProduceMailbox(userInvoke func(interface{}), systemInvoke func(interfaces.SystemMessage)) interfaces.Mailbox {
	return props.mailboxProducer(userInvoke, systemInvoke)
}

func Props(actorProducer interfaces.ActorProducer) PropsValue {
	return PropsValue{
		actorProducer: actorProducer,
		mailboxProducer: mailbox.NewQueueMailbox,
	}
}

func (props PropsValue) WithMailbox(mailboxProducer interfaces.MailboxProducer) PropsValue {
	//pass by value, we only modify the copy
	props.mailboxProducer = mailboxProducer
	return props
}
