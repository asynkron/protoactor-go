package routing

type ManagementMessage interface {
	RouterManagementMessage()
}

type BroadcastMessage struct {
	Message interface{}
}

func (*AddRoutee) RouterManagementMessage()        {}
func (*RemoveRoutee) RouterManagementMessage()     {}
func (*GetRoutees) RouterManagementMessage()       {}
func (*AdjustPoolSize) RouterManagementMessage()   {}
func (*BroadcastMessage) RouterManagementMessage() {}
