package cluster

import (
	"net"
	"strconv"
	"strings"
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
