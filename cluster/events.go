package cluster

import "github.com/hashicorp/memberlist"

type eventDelegate struct {
}

type clusterStatusJoin struct {
	node *clusterNode
}

type clusterStatusLeave struct {
	node *clusterNode
}

func (*eventDelegate) NotifyJoin(node *memberlist.Node) {
	cn := getClusterNode(node)
	clusterPid.Tell(&clusterStatusJoin{node: cn})
}

func (*eventDelegate) NotifyLeave(node *memberlist.Node) {
	cn := getClusterNode(node)
	clusterPid.Tell(&clusterStatusLeave{node: cn})
}

func (*eventDelegate) NotifyUpdate(node *memberlist.Node) {
	//cn := getClusterNode(node)
}

func newEventDelegate() memberlist.EventDelegate {
	return &eventDelegate{}
}
