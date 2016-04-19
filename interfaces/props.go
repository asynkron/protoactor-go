package interfaces

type Props interface {
	ProduceActor() Actor
	Mailbox() Mailbox
	Supervisor() SupervisionStrategy
}
