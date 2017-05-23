package gocb

import (
	"gopkg.in/couchbase/gocbcore.v2"
)

func (b *Bucket) observeOnceCas(key []byte, cas Cas, forDelete bool, replicaIdx int, commCh chan uint) (pendingOp, error) {
	return b.client.Observe(key, replicaIdx,
		func(ks gocbcore.KeyState, obsCas gocbcore.Cas, err error) {
			if err != nil {
				commCh <- 0
				return
			}

			didReplicate := false
			didPersist := false

			if ks == gocbcore.KeyStatePersisted {
				if !forDelete {
					if Cas(obsCas) == cas {
						if replicaIdx != 0 {
							didReplicate = true
						}
						didPersist = true
					}
				}
			} else if ks == gocbcore.KeyStateNotPersisted {
				if !forDelete {
					if Cas(obsCas) == cas {
						if replicaIdx != 0 {
							didReplicate = true
						}
					}
				}
			} else if ks == gocbcore.KeyStateDeleted {
				if forDelete {
					didReplicate = true
				}
			} else {
				if forDelete {
					didReplicate = true
					didPersist = true
				}
			}

			var out uint
			if didReplicate {
				out |= 1
			}
			if didPersist {
				out |= 2
			}
			commCh <- out
		})
}

func (b *Bucket) observeOnceSeqNo(key []byte, mt MutationToken, replicaIdx int, commCh chan uint) (pendingOp, error) {
	return b.client.ObserveSeqNo(key, mt.token.VbUuid, replicaIdx,
		func(currentSeqNo gocbcore.SeqNo, persistSeqNo gocbcore.SeqNo, err error) {
			if err != nil {
				commCh <- 0
				return
			}

			didReplicate := currentSeqNo >= mt.token.SeqNo
			didPersist := persistSeqNo >= mt.token.SeqNo

			var out uint
			if didReplicate {
				out |= 1
			}
			if didPersist {
				out |= 2
			}
			commCh <- out
		})
}

func (b *Bucket) observeOne(key []byte, mt MutationToken, cas Cas, forDelete bool, replicaIdx int, replicaCh, persistCh chan bool) {
	observeOnce := func(commCh chan uint) (pendingOp, error) {
		if mt.token.VbUuid != 0 && mt.token.SeqNo != 0 {
			return b.observeOnceSeqNo(key, mt, replicaIdx, commCh)
		}
		return b.observeOnceCas(key, cas, forDelete, replicaIdx, commCh)
	}

	sentReplicated := false
	sentPersisted := false

	failMe := func() {
		if !sentReplicated {
			replicaCh <- false
			sentReplicated = true
		}
		if !sentPersisted {
			persistCh <- false
			sentPersisted = true
		}
	}

	timeoutTmr := gocbcore.AcquireTimer(b.duraTimeout)

	commCh := make(chan uint)
	for {
		op, err := observeOnce(commCh)
		if err != nil {
			gocbcore.ReleaseTimer(timeoutTmr, false)
			failMe()
			return
		}

		select {
		case val := <-commCh:
			// Got Value
			if (val&1) != 0 && !sentReplicated {
				replicaCh <- true
				sentReplicated = true
			}
			if (val&2) != 0 && !sentPersisted {
				persistCh <- true
				sentPersisted = true
			}

			waitTmr := gocbcore.AcquireTimer(b.duraPollTimeout)
			select {
			case <-waitTmr.C:
				gocbcore.ReleaseTimer(waitTmr, true)
				// Fall through to outside for loop
			case <-timeoutTmr.C:
				gocbcore.ReleaseTimer(waitTmr, false)
				gocbcore.ReleaseTimer(timeoutTmr, true)
				failMe()
				return
			}

		case <-timeoutTmr.C:
			// Timed out
			op.Cancel()
			gocbcore.ReleaseTimer(timeoutTmr, true)
			failMe()
			return
		}
	}
}

func (b *Bucket) durability(key string, cas Cas, mt MutationToken, replicaTo, persistTo uint, forDelete bool) error {
	numServers := b.client.NumReplicas() + 1

	if replicaTo > uint(numServers-1) || persistTo > uint(numServers) {
		return ErrNotEnoughReplicas
	}

	keyBytes := []byte(key)

	replicaCh := make(chan bool)
	persistCh := make(chan bool)

	for replicaIdx := 0; replicaIdx < numServers; replicaIdx++ {
		go b.observeOne(keyBytes, mt, cas, forDelete, replicaIdx, replicaCh, persistCh)
	}

	results := int(0)
	replicas := uint(0)
	persists := uint(0)

	for {
		select {
		case rV := <-replicaCh:
			if rV {
				replicas++
			}
			results++
		case pV := <-persistCh:
			if pV {
				persists++
			}
			results++
		}

		if replicas >= replicaTo && persists >= persistTo {
			return nil
		} else if results == ((numServers * 2) - 1) {
			return ErrDurabilityTimeout
		}
	}
}

// Touches a document, specifying a new expiry time for it.  Additionally checks document durability.
func (b *Bucket) TouchDura(key string, cas Cas, expiry uint32, replicateTo, persistTo uint) (Cas, error) {
	cas, mt, err := b.touch(key, cas, expiry)
	if err != nil {
		return cas, err
	}
	return cas, b.durability(key, cas, mt, replicateTo, persistTo, false)
}

// Removes a document from the bucket.  Additionally checks document durability.
func (b *Bucket) RemoveDura(key string, cas Cas, replicateTo, persistTo uint) (Cas, error) {
	cas, mt, err := b.remove(key, cas)
	if err != nil {
		return cas, err
	}
	return cas, b.durability(key, cas, mt, replicateTo, persistTo, true)
}

// Inserts or replaces a document in the bucket.  Additionally checks document durability.
func (b *Bucket) UpsertDura(key string, value interface{}, expiry uint32, replicateTo, persistTo uint) (Cas, error) {
	cas, mt, err := b.upsert(key, value, expiry)
	if err != nil {
		return cas, err
	}
	return cas, b.durability(key, cas, mt, replicateTo, persistTo, false)
}

// Inserts a new document to the bucket.  Additionally checks document durability.
func (b *Bucket) InsertDura(key string, value interface{}, expiry uint32, replicateTo, persistTo uint) (Cas, error) {
	cas, mt, err := b.insert(key, value, expiry)
	if err != nil {
		return cas, err
	}
	return cas, b.durability(key, cas, mt, replicateTo, persistTo, false)
}

// Replaces a document in the bucket.  Additionally checks document durability.
func (b *Bucket) ReplaceDura(key string, value interface{}, cas Cas, expiry uint32, replicateTo, persistTo uint) (Cas, error) {
	cas, mt, err := b.replace(key, value, cas, expiry)
	if err != nil {
		return cas, err
	}
	return cas, b.durability(key, cas, mt, replicateTo, persistTo, false)
}

// Appends a string value to a document.  Additionally checks document durability.
func (b *Bucket) AppendDura(key, value string, replicateTo, persistTo uint) (Cas, error) {
	cas, mt, err := b.append(key, value)
	if err != nil {
		return cas, err
	}
	return cas, b.durability(key, cas, mt, replicateTo, persistTo, false)
}

// Prepends a string value to a document.  Additionally checks document durability.
func (b *Bucket) PrependDura(key, value string, replicateTo, persistTo uint) (Cas, error) {
	cas, mt, err := b.prepend(key, value)
	if err != nil {
		return cas, err
	}
	return cas, b.durability(key, cas, mt, replicateTo, persistTo, false)
}

// Performs an atomic addition or subtraction for an integer document.  Additionally checks document durability.
func (b *Bucket) CounterDura(key string, delta, initial int64, expiry uint32, replicateTo, persistTo uint) (uint64, Cas, error) {
	val, cas, mt, err := b.counter(key, delta, initial, expiry)
	if err != nil {
		return val, cas, err
	}
	return val, cas, b.durability(key, cas, mt, replicateTo, persistTo, false)
}
