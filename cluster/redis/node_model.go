package redis

import "fmt"

// NodeModel represents a node in the cluster
type NodeModel struct {
	ID      string   `json:"id"`
	Address string   `json:"address"`
	Port    int      `json:"port"`
	Kinds   []string `json:"kinds"`
}

// NewNode returns a new node for the cluster
func NewNode(clusterName string, address string, port int, kind []string) *NodeModel {
	return &NodeModel{
		ID:      createNodeID(clusterName, address, port),
		Address: address,
		Port:    port,
		Kinds:   kind,
	}
}

func createNodeID(clusterName string, address string, port int) string {
	return fmt.Sprintf("%v/%v:%v", clusterName, address, port)
}
