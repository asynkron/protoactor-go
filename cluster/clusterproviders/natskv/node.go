package natskv

import (
	"encoding/json"

	"github.com/awevoke/protoactor-go/cluster"
)

// Node represents a cluster member stored in NATS KV.
type Node struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Host  string   `json:"host"`
	Port  int      `json:"port"`
	Kinds []string `json:"kinds"`
	Alive bool     `json:"alive"`
}

// NewNode constructs a new Node instance with Alive set to true.
func NewNode(name, host string, port int, kinds []string) *Node {
	return &Node{
		ID:    name,
		Name:  name,
		Host:  host,
		Port:  port,
		Kinds: kinds,
		Alive: true,
	}
}

// NewNodeFromBytes decodes a Node from its JSON representation.
func NewNodeFromBytes(data []byte) (*Node, error) {
	n := &Node{}
	if err := json.Unmarshal(data, n); err != nil {
		return nil, err
	}
	return n, nil
}

// Equal compares two nodes by ID. Returns false if either node is nil.
func (n *Node) Equal(other *Node) bool {
	if n == nil || other == nil {
		return false
	}
	if n == other {
		return true
	}
	return n.ID == other.ID
}

// MemberStatus converts the node into a cluster.Member description.
func (n *Node) MemberStatus() *cluster.Member {
	kinds := n.Kinds
	if kinds == nil {
		kinds = []string{}
	}
	return &cluster.Member{
		Id:    n.ID,
		Host:  n.Host,
		Port:  int32(n.Port),
		Kinds: kinds,
	}
}

// IsAlive reports whether the node is considered alive.
func (n *Node) IsAlive() bool {
	return n.Alive
}

// SetAlive updates the alive flag for the node.
func (n *Node) SetAlive(alive bool) {
	n.Alive = alive
}

// Serialize encodes the node to JSON.
func (n *Node) Serialize() ([]byte, error) {
	return json.Marshal(n)
}

// Deserialize populates the node from JSON data.
func (n *Node) Deserialize(data []byte) error {
	return json.Unmarshal(data, n)
}
