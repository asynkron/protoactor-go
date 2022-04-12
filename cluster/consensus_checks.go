// Copyright (C) 2015-2022 Asynkron AB All rights reserved

package cluster

type GossipUpdater func(*GossipState, map[string]empty)

// data structure helpful to store consensus check information and behavior.
type ConsensusCheck struct {
	affectedKeys []string
	check        GossipUpdater
}

// creates a new ConsensusCheck value with the given data and return it back.
func NewConsensusCheck(affectedKeys []string, check GossipUpdater) ConsensusCheck {
	consensusCheck := ConsensusCheck{
		affectedKeys: affectedKeys,
		check:        check,
	}

	return consensusCheck
}

// acts as a storage of pointers to ConsensusCheck stored by key.
type ConsensusChecks struct {
	checks                 map[string]*ConsensusCheck
	affectedKeysByStateKey map[string]map[string]empty
}

// creates a new ConsensusChecks value and returns a pointer to it.
func NewConsensusChecks() *ConsensusChecks {
	checks := ConsensusChecks{
		checks:                 make(map[string]*ConsensusCheck),
		affectedKeysByStateKey: make(map[string]map[string]empty),
	}

	return &checks
}

// iterates over all the keys stored in the set (map[string]empty) found in
// the given key map and populates a slice of pointers to ConsensusCheck values
// that is returned as a set of ConsensusCheck updated by the given key.
func (cc *ConsensusChecks) GetByUpdatedKey(key string) []*ConsensusCheck {
	var result []*ConsensusCheck
	if _, ok := cc.affectedKeysByStateKey[key]; !ok {
		return result
	}

	for id := range cc.affectedKeysByStateKey[key] {
		if ConsensusCheck, ok := cc.checks[id]; ok {
			result = append(result, ConsensusCheck)
		}
	}

	return result
}

// iterate over all the keys stored in the set (map[string]empty) found in
// the given key maps and populates a slice of pointers to ConsensusCheck values
// that is returned as a set of ConsensusCheck updated by the given keys
// with removed duplicates on it (as it is a "set").
func (cc *ConsensusChecks) GetByUpdatedKeys(keys []string) []*ConsensusCheck {
	var result []*ConsensusCheck

	temporaryIDs := make(map[string]empty)

	for _, key := range keys {
		if _, ok := cc.affectedKeysByStateKey[key]; !ok {
			continue
		}

		for id := range cc.affectedKeysByStateKey[key] {
			if ConsensusCheck, ok := cc.checks[id]; ok {
				if _, ok := temporaryIDs[id]; !ok {
					temporaryIDs[id] = empty{}

					result = append(result, ConsensusCheck)
				}
			}
		}
	}

	return result
}

// adds a new pointer to a ConsensusCheck value in the storage
// and registers its affected by keys index.
func (cc *ConsensusChecks) Add(id string, check *ConsensusCheck) {
	cc.checks[id] = check
	cc.registerAffectedKeys(id, check.affectedKeys)
}

// Remove removes the given ConsensusCheck identity from the storage and
// removes its affected by keys index if needed after cleaning.
func (cc *ConsensusChecks) Remove(id string) {
	if _, ok := cc.affectedKeysByStateKey[id]; ok {
		delete(cc.affectedKeysByStateKey, id)
		cc.unregisterAffectedKeys(id)
	}
}

func (cc *ConsensusChecks) registerAffectedKeys(id string, keys []string) {
	for _, key := range keys {
		if _, ok := cc.affectedKeysByStateKey[key]; ok {
			cc.affectedKeysByStateKey[key][id] = empty{}
		} else {
			cc.affectedKeysByStateKey[key] = map[string]empty{id: {}}
		}
	}
}

func (cc *ConsensusChecks) unregisterAffectedKeys(id string) {
	var keysToDelete []string

	for key, internal := range cc.affectedKeysByStateKey {
		if _, ok := internal[id]; ok {
			delete(cc.affectedKeysByStateKey[key], id)

			if len(cc.affectedKeysByStateKey[key]) == 0 {
				keysToDelete = append(keysToDelete, key)
			}
		}
	}

	for _, key := range keysToDelete {
		delete(cc.affectedKeysByStateKey, key)
	}
}
