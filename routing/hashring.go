package routing

import (
	"github.com/serialx/hashring"
)

type Hashring struct {
	ring *hashring.HashRing
	Hasher
}

func New() *Hashring {
	h := &Hashring{}
	return h
}

func (h *Hashring) GetNode(message Hashable) (string, error) {
	if h.ring == nil {
		panic("Hashring is not initialized")
	}
	if key, ok := h.ring.GetNode(message.HashBy()); ok {
		return key, nil
	}
	return "", ErrorUnknownPartition
}

func (h *Hashring) SetNodes(nodes []string) {
	h.ring = hashring.New(nodes)
}

func (h *Hashring) AddNode(node string) {
	h.ring.AddNode(node)
}

func (h *Hashring) RemoveNode(node string) {
	h.ring.RemoveNode(node)
}
