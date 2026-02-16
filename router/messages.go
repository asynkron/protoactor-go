package router

type ManagementMessage interface {
	ManagementMessage()
}

type BroadcastMessage struct {
	Message any
}

func (*AddRoutee) ManagementMessage()        {}
func (*RemoveRoutee) ManagementMessage()     {}
func (*GetRoutees) ManagementMessage()       {}
func (*AdjustPoolSize) ManagementMessage()   {}
func (*BroadcastMessage) ManagementMessage() {}
