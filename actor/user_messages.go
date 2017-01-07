package actor

type NotInfluenceReceiveTimeout interface {
	NotInfluenceReceiveTimeout()
}

var (
	restartingMessage     interface{} = &Restarting{}
	stoppingMessage       interface{} = &Stopping{}
	stoppedMessage        interface{} = &Stopped{}
	poisonPillMessage     interface{} = &PoisonPill{}
	startedMessage        interface{} = &Started{}
	receiveTimeoutMessage interface{} = &ReceiveTimeout{}
)
