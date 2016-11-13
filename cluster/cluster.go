package cluster

import (
	"fmt"
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting"
	"github.com/hashicorp/memberlist"
)

var list *memberlist.Memberlist

//Start the cluster and optionally join other nodes
func Start(ip string, join ...string) {
	c := memberlist.DefaultLocalConfig()
	h, p := getAddress(ip)
	log.Printf("Starting on %v:%v", h, p)
	if p == 0 {
		p = findFreePort()
	}
	c.BindPort = p
	c.BindAddr = h
	c.Name = fmt.Sprintf("%v:%v", h, p+1)
	gossiper := NewMemberlistGossiper(c.Name)
	c.Delegate = gossiper

	l, err := memberlist.Create(c)
	remoting.Start(fmt.Sprintf("%v:%v", h, p+1))

	if err != nil {
		panic("Failed to create memberlist: " + err.Error())
	}
	list = l

	if len(join) > 0 {
		// Join an existing cluster by specifying at least one known member.
		_, err = list.Join(join)
		if err != nil {
			panic("Failed to join cluster: " + err.Error())
		}
	}

	actor.SpawnNamed(actor.FromProducer(newClusterActor(list)), "cluster")
	actor.SpawnNamed(actor.FromProducer(newActivatorActor()), "activator")
}
