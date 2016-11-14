package cluster

import (
	"fmt"
	"log"

	"github.com/AsynkronIT/gam/remoting"
	"github.com/hashicorp/memberlist"
)

var list *memberlist.Memberlist

//Start the cluster and optionally join other nodes
func Start(ip string, join ...string) {
	h, p := getAddress(ip)
	log.Printf("Starting on %v:%v", h, p)
	if p == 0 {
		p = findFreePort()
	}
	name := fmt.Sprintf("%v:%v", h, p+1)
	c := getMemberlistConfig(h, p, name)
	l, err := memberlist.Create(c)

	if err != nil {
		panic("Failed to create memberlist: " + err.Error())
	}
	list = l
	remoting.Start(name)

	if len(join) > 0 {
		// Join an existing cluster by specifying at least one known member.
		_, err = list.Join(join)
		if err != nil {
			panic("Failed to join cluster: " + err.Error())
		}
	}
}
