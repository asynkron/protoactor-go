package natsstream

import (
	"encoding/json"

	"github.com/asynkron/protoactor-go/cluster"
)

// Node represents a cluster member stored in a JetStream stream message.
type Node struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Host  string   `json:"host"`
	Port  int      `json:"port"`
	Kinds []string `json:"kinds"`
	Alive bool     `json:"alive"`
}

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

func NewNodeFromBytes(data []byte) (*Node, error) {
	n := &Node{}
	if err := json.Unmarshal(data, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *Node) Equal(other *Node) bool {
	if n == nil || other == nil {
		return false
	}
	if n == other {
		return true
	}
	return n.ID == other.ID
}

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

func (n *Node) IsAlive() bool { return n.Alive }
func (n *Node) SetAlive(alive bool) { n.Alive = alive }

func (n *Node) Serialize() ([]byte, error) { return json.Marshal(n) }
func (n *Node) Deserialize(data []byte) error { return json.Unmarshal(data, n) }
