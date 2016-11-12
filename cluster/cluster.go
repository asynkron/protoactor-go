package cluster

import (
	"fmt"
	"log"
	"os"

	"github.com/AsynkronIT/gam/actor"
	"github.com/hashicorp/memberlist"
)

func nodeName(prefix string, port int) string {
	h, _ := os.Hostname()
	return fmt.Sprintf("%v_%v:%v", prefix, h, port)
}

var list *memberlist.Memberlist

func Start(ip string, join ...string) {
	c := memberlist.DefaultLocalConfig()
	h, p := getAddress(ip)
	log.Printf("Starting on %v:%v", h, p)
	if p == 0 {
		p = findFreePort()
	}
	c.BindPort = p
	c.BindAddr = h
	c.Name = nodeName("member", c.BindPort)
	gossiper := NewMemberlistGossiper(c.Name)
	c.Delegate = gossiper

	l, err := memberlist.Create(c)

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

	props := actor.FromProducer(newClusterActor(list))
	actor.SpawnNamed(props, "cluster")

	// // Ask for members of the cluster
	// for _, member := range list.Members() {
	// 	log.Printf("Member: %s %s\n", member.Name, member.Addr)
	// }

}
