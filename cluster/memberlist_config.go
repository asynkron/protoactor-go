package cluster

import "github.com/hashicorp/memberlist"

func getMemberlistConfig(h string, p int, name string) *memberlist.Config {
	c := memberlist.DefaultLocalConfig()
	c.BindPort = p
	c.BindAddr = h
	c.Name = name
	c.Delegate = NewMemberlistGossiper(c.Name)
	c.Events = newEventDelegate()
	return c
}
