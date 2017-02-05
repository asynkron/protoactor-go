package gocbcore

// A type representing a unique revision of a document.
// This can be used to perform optimistic locking.
type Cas uint64

// A unique identifier for a particular vbucket history.
type VbUuid uint64

// A sequential mutation number indicating the order and
// precise position of a write that has occured.
type SeqNo uint64

// Represents a particular mutation within the cluster.
type MutationToken struct {
	VbId   uint16
	VbUuid VbUuid
	SeqNo  SeqNo
}

// Represents the stats returned from a single server.
type SingleServerStats struct {
	Stats map[string]string
	Error error
}

// The results from a single subdocument operation.
type SubDocResult struct {
	Err   error
	Value []byte
}

// Represents an outstanding operation within the client.
// This can be used to cancel an operation before it completes.
type PendingOp interface {
	Cancel() bool
}

type multiPendingOp struct {
	ops []PendingOp
}

func (mp *multiPendingOp) Cancel() bool {
	allCancelled := true
	for _, op := range mp.ops {
		if !op.Cancel() {
			allCancelled = false
		}
	}
	return allCancelled
}

func (c *Agent) dispatchOp(req *memdQRequest) (PendingOp, error) {
	err := c.dispatchDirect(req)
	if err != nil {
		return nil, err
	}
	return req, nil
}

type GetCallback func([]byte, uint32, Cas, error)
type UnlockCallback func(Cas, MutationToken, error)
type TouchCallback func(Cas, MutationToken, error)
type RemoveCallback func(Cas, MutationToken, error)
type StoreCallback func(Cas, MutationToken, error)
type CounterCallback func(uint64, Cas, MutationToken, error)
type ObserveCallback func(KeyState, Cas, error)
type ObserveSeqNoCallback func(SeqNo, SeqNo, error)
type GetRandomCallback func([]byte, []byte, uint32, Cas, error)
type ServerStatsCallback func(stats map[string]SingleServerStats)
type GetInCallback func([]byte, Cas, error)
type ExistsInCallback func(Cas, error)
type RemoveInCallback func(Cas, MutationToken, error)
type StoreInCallback func(Cas, MutationToken, error)
type CounterInCallback func([]byte, Cas, MutationToken, error)
type LookupInCallback func([]SubDocResult, Cas, error)
type MutateInCallback func([]SubDocResult, Cas, MutationToken, error)
