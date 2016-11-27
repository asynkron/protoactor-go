package cluster

import "github.com/hashicorp/memberlist"

var (
	list *memberlist.Memberlist
)

func getMemberlistConfig(host string, port int, name string) *memberlist.Config {
	c := memberlist.DefaultLocalConfig()
	c.BindPort = port
	c.BindAddr = host
	c.Name = name
	c.Delegate = newMemberlistGossiper(c.Name)
	c.Events = newEventDelegate()
	return c
}
