package cluster

import (
	"fmt"
	"log"
	"time"

	"github.com/AsynkronIT/gam/remoting"
	"github.com/AsynkronIT/gonet"
	"github.com/hashicorp/memberlist"
)

//Start the cluster and optionally join other nodes
func Start(ip string, join ...string) {
	h, p := gonet.GetAddress(ip)
	log.Printf("[CLUSTER] Starting on %v:%v", h, p)
	if p == 0 {
		p = gonet.FindFreePort()
	}
	name := fmt.Sprintf("%v:%v", h, p+1)
	c := getMemberlistConfig(h, p, name)
	l, err := memberlist.Create(c)

	if err != nil {
		panic("[CLUSTER] Failed to create memberlist: " + err.Error())
	}
	list = l
	remoting.Start(name)

	if len(join) > 0 {
		// Join an existing cluster by specifying at least one known member.
		_, err = list.Join(join)
		if err != nil {
			panic("[CLUSTER] Failed to join cluster: " + err.Error())
		}
		time.Sleep(500 * time.Millisecond)
	}
}
