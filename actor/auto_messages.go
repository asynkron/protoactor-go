package actor

type AutoReceiveMessage interface {
	AutoReceiveMessage()
}

type Restarting struct{}

func (*Restarting) AutoReceiveMessage() {}

type Stopping struct{}

func (*Stopping) AutoReceiveMessage() {}

type Stopped struct{}

func (*Stopped) AutoReceiveMessage()    {}
func (*PoisonPill) AutoReceiveMessage() {}

type Started struct{}

func (*Started) AutoReceiveMessage() {}

type ReceiveTimeout struct{}
