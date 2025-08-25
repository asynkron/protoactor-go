package etcd

import (
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/asynkron/protoactor-go/cluster"
)

const (
	metaKeyID  = "id"
	metaKeySeq = "seq"
)

// Node represents a cluster member stored in etcd.
type Node struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Host    string            `json:"host"`
	Address string            `json:"address"`
	Port    int               `json:"port"`
	Kinds   []string          `json:"kinds"`
	Meta    map[string]string `json:"-"`
	Alive   bool              `json:"alive"`
}

// NewNode constructs a new Node instance.
func NewNode(name, host string, port int, kinds []string) *Node {
	return &Node{
		ID:      name,
		Name:    name,
		Address: host,
		Host:    host,
		Port:    port,
		Kinds:   kinds,
		Meta:    map[string]string{},
		Alive:   true,
	}
}

// NewNodeFromBytes decodes a Node from its JSON representation.
func NewNodeFromBytes(data []byte) (*Node, error) {
	n := Node{}
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// GetAddress returns the host and port for the node.
func (n *Node) GetAddress() (host string, port int) {
	host = n.Host
	port = n.Port
	if host == "" {
		host = n.Address
	}
	return
}

// Equal compares two nodes by ID.
func (n *Node) Equal(other *Node) bool {
	if n == nil || other == nil {
		return false
	}
	if n == other {
		return true
	}
	return n.ID == other.ID
}

// GetMeta returns a metadata value by name.
func (n *Node) GetMeta(name string) (string, bool) {
	if n.Meta == nil {
		return "", false
	}
	val, ok := n.Meta[name]
	return val, ok
}

// GetSeq returns the sequence number from metadata.
func (n *Node) GetSeq() int {
	if seqStr, ok := n.GetMeta(metaKeySeq); ok {
		return strToInt(seqStr)
	}
	return 0
}

// strToInt converts a string to an int, logging and returning 0 on failure.
func strToInt(s string) int {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		slog.Error("failed to parse int", slog.String("value", s), slog.Any("error", err))
		return 0
	}
	return int(i)
}

// MemberStatus converts the node into a cluster.Member description.
func (n *Node) MemberStatus() *cluster.Member {
	host, port := n.GetAddress()
	kinds := n.Kinds
	if kinds == nil {
		kinds = []string{}
	}
	return &cluster.Member{
		Id:    n.ID,
		Host:  host,
		Port:  int32(port),
		Kinds: kinds,
	}
}

// SetMeta sets a metadata value.
func (n *Node) SetMeta(name string, val string) {
	if n.Meta == nil {
		n.Meta = map[string]string{}
	}
	n.Meta[name] = val
}

// Serialize encodes the node to JSON.
func (n *Node) Serialize() ([]byte, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Deserialize populates the node from JSON data.
func (n *Node) Deserialize(data []byte) error {
	return json.Unmarshal(data, n)
}

// IsAlive reports whether the node is considered alive.
func (n *Node) IsAlive() bool {
	return n.Alive
}

// SetAlive updates the alive flag for the node.
func (n *Node) SetAlive(alive bool) {
	n.Alive = alive
}
