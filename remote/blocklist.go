/*
	Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>
*/

package remote

import (
	"sync"

	"github.com/asynkron/gofun/set"
)

// TODO: document it
type BlockList struct {
	mu             *sync.RWMutex
	blockedMembers *set.ImmutableSet[string]
}

func NewBlockList() *BlockList {
	blocklist := BlockList{
		mu:             &sync.RWMutex{},
		blockedMembers: set.NewImmutable[string](),
	}
	return &blocklist
}

func (bl *BlockList) BlockedMembers() set.Set[string] {
	return bl.blockedMembers
}

// Block adds the given memberID list to the BlockList
func (bl *BlockList) Block(memberIDs ...string) {
	// acquire our mutual exclusion primitive
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.blockedMembers = bl.blockedMembers.AddRange(memberIDs...)
}

// IsBlocked returns true if the given memberID string has been
// ever added to the BlockList
func (bl *BlockList) IsBlocked(memberID string) bool {
	// acquire our mutual exclusion primitive for reading
	return bl.blockedMembers.Contains(memberID)
}

// Len returns the number of blocked members
func (bl *BlockList) Len() int {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return bl.blockedMembers.Size()
}
