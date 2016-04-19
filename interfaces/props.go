package interfaces

type Props interface {
	ProduceActor() Actor
	ProduceMailbox(userInvoke func(interface{}), systemInvoke func(SystemMessage)) Mailbox
	Supervisor() SupervisionStrategy
}
