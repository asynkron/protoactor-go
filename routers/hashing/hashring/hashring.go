package hashring

import (
	"github.com/AsynkronIT/gam/actor"
	"github.com/serialx/hashring"
)

type Hashring struct {
	ring *hashring.HashRing
	actor.Hasher
}

func New() *Hashring {
	h := &Hashring{}
	return h
}

func (h *Hashring) GetNode(message actor.Hashable) (string, error) {
	if h.ring == nil {
		panic("Hashring is not initialized")
	}
	if key, ok := h.ring.GetNode(message.HashBy()); ok {
		return key, nil
	}
	return "", actor.ErrorUnknownPartition
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
