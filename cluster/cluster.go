package cluster

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/memberlist"
)

func findFreePort() int {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	s := l.Addr().String()
	_, p := getAddress(s)
	return p
}

func getAddress(addr string) (string, int) {
	i := strings.LastIndex(addr, ":")
	p, _ := strconv.Atoi(addr[i+1 : len(addr)])
	h := addr[0:i]
	return h, p
}

func nodeName(prefix string, port int) string {
	h, _ := os.Hostname()
	return fmt.Sprintf("%v_%v:%v", prefix, h, port)
}

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

	list, err := memberlist.Create(c)
	if err != nil {
		panic("Failed to create memberlist: " + err.Error())
	}

	if len(join) > 0 {
		// Join an existing cluster by specifying at least one known member.
		_, err = list.Join(join)
		if err != nil {
			panic("Failed to join cluster: " + err.Error())
		}
	}

	// Ask for members of the cluster
	for _, member := range list.Members() {
		log.Printf("Member: %s %s\n", member.Name, member.Addr)
	}
}
