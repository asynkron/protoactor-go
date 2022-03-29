/*
	Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>
*/

package remote

import "sync"

type empty struct{}

// TODO: document it
type BlockList struct {
	mu             *sync.RWMutex
	blockedMembers map[string]empty
}

func NewBlockList() BlockList {

	blocklist := BlockList{
		mu:             &sync.RWMutex{},
		blockedMembers: make(map[string]empty),
	}
	return blocklist
}

// Returns back a copy of the internal blockedMembers map
func (bl *BlockList) BlockedMembers() map[string]struct{} {

	// we don't allow external users to mutate our map, so make a copy
	blocked := make(map[string]struct{})
	for k := range bl.blockedMembers {
		blocked[k] = struct{}{}
	}

	return blocked
}

// Block adds the given memberID list to the BlockList
func (bl *BlockList) Block(memberIDs ...string) {

	// acquire our mutual exclusion primitive
	bl.mu.Lock()
	defer bl.mu.Unlock()

	for _, memberID := range memberIDs {
		bl.blockedMembers[memberID] = empty{}
	}
}

// IsBlocked returns true if the given memberID string has been
// ever added to the BlockList
func (bl *BlockList) IsBlocked(memberID string) bool {

	// acquire our mutual exclusion primitive for reading
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	_, ok := bl.blockedMembers[memberID]
	return ok
}
