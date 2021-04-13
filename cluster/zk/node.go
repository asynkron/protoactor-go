package zk

import (
	"encoding/json"
	"fmt"

	"github.com/AsynkronIT/protoactor-go/cluster"
)

const (
	metaKeyID  = "id"
	metaKeySeq = "seq"
)

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

func (n *Node) GetAddress() (host string, port int) {
	host = n.Host
	port = n.Port
	if host == "" {
		host = n.Address
	}
	return
}

func (n *Node) GetAddressString() string {
	h, p := n.GetAddress()
	return fmt.Sprintf("%v:%v", h, p)
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

func (n *Node) GetMeta(name string) (string, bool) {
	if n.Meta == nil {
		return "", false
	}
	val, ok := n.Meta[name]
	return val, ok
}

func (n *Node) GetSeq() int {
	if seqStr, ok := n.GetMeta(metaKeySeq); ok {
		return strToInt(seqStr)
	}
	return 0
}

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

func (n *Node) SetMeta(name string, val string) {
	if n.Meta == nil {
		n.Meta = map[string]string{}
	}
	n.Meta[name] = val
}

func (n *Node) Serialize() ([]byte, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (n *Node) Deserialize(data []byte) error {
	return json.Unmarshal(data, n)
}
