package actor

type AutoReceiveMessage interface {
	AutoReceiveMessage()
}

func (*Restarting) AutoReceiveMessage() {}
func (*Stopping) AutoReceiveMessage()   {}
func (*Stopped) AutoReceiveMessage()    {}
func (*PoisonPill) AutoReceiveMessage() {}
func (*Started) AutoReceiveMessage()    {}
