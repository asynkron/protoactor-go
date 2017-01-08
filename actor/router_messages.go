package actor

type RouterManagementMessage interface {
	RouterManagementMessage()
}

type RouterBroadcastMessage struct {
	Message interface{}
}

func (*RouterAddRoutee) RouterManagementMessage()        {}
func (*RouterRemoveRoutee) RouterManagementMessage()     {}
func (*RouterGetRoutees) RouterManagementMessage()       {}
func (*RouterAdjustPoolSize) RouterManagementMessage()   {}
func (*RouterBroadcastMessage) RouterManagementMessage() {}
