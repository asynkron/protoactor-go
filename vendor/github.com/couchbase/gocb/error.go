package gocb

import (
	"errors"
	"gopkg.in/couchbase/gocbcore.v2"
)

type clientError struct {
	message string
}

func (e clientError) Error() string {
	return e.message
}

var (
	// ErrNotEnoughReplicas occurs when not enough replicas exist to match the specified durability requirements.
	ErrNotEnoughReplicas = errors.New("Not enough replicas to match durability requirements.")
	// ErrDurabilityTimeout occurs when the server took too long to meet the specified durability requirements.
	ErrDurabilityTimeout = errors.New("Failed to meet durability requirements in time.")
	// ErrNoResults occurs when no results are available to a query.
	ErrNoResults = errors.New("No results returned.")
	// ErrNoOpenBuckets occurs when a cluster-level operation is performed before any buckets are opened.
	ErrNoOpenBuckets = errors.New("You must open a bucket before you can perform cluster level operations.")
	// ErrIndexInvalidName occurs when an invalid name was specified for an index.
	ErrIndexInvalidName = errors.New("An invalid index name was specified.")
	// ErrIndexNoFields occurs when an index with no fields is created.
	ErrIndexNoFields = errors.New("You must specify at least one field to index.")
	// ErrIndexNotFound occurs when an operation expects an index but it was not found.
	ErrIndexNotFound = errors.New("The index specified does not exist.")
	// ErrIndexAlreadyExists occurs when an operation expects an index not to exist, but it was found.
	ErrIndexAlreadyExists = errors.New("The index specified already exists.")
	// ErrFacetNoRanges occurs when a range-based facet is specified but no ranges were indicated.
	ErrFacetNoRanges = errors.New("At least one range must be specified on a facet.")

	// ErrDispatchFail occurs when we failed to execute an operation due to internal routing issues.
	ErrDispatchFail = gocbcore.ErrDispatchFail
	// ErrBadHosts occurs when an invalid list of hosts is specified for bootstrapping.
	ErrBadHosts = gocbcore.ErrBadHosts
	// ErrProtocol occurs when an invalid protocol is specified for bootstrapping.
	ErrProtocol = gocbcore.ErrProtocol
	// ErrNoReplicas occurs when an operation expecting replicas is performed, but no replicas are available.
	ErrNoReplicas = gocbcore.ErrNoReplicas
	// ErrInvalidServer occurs when a specified server index is invalid.
	ErrInvalidServer = gocbcore.ErrInvalidServer
	// ErrInvalidVBucket occurs when a specified vbucket index is invalid.
	ErrInvalidVBucket = gocbcore.ErrInvalidVBucket
	// ErrInvalidReplica occurs when a specified replica index is invalid.
	ErrInvalidReplica = gocbcore.ErrInvalidReplica

	// ErrInvalidCert occurs when the specified certificate is not valid.
	ErrInvalidCert = gocbcore.ErrInvalidCert

	// ErrShutdown occurs when an operation is performed on a bucket that has been closed.
	ErrShutdown = gocbcore.ErrShutdown
	// ErrOverload occurs when more operations were dispatched than the client is capable of writing.
	ErrOverload = gocbcore.ErrOverload
	// ErrNetwork occurs when various generic network errors occur.
	ErrNetwork = gocbcore.ErrNetwork
	// ErrTimeout occurs when an operation times out.
	ErrTimeout = gocbcore.ErrTimeout

	// ErrStreamClosed occurs when an error is related to a stream closing.
	ErrStreamClosed = gocbcore.ErrStreamClosed
	// ErrStreamStateChanged occurs when an error is related to a cluster rebalance.
	ErrStreamStateChanged = gocbcore.ErrStreamStateChanged
	// ErrStreamDisconnected occurs when a stream is closed due to a connection dropping.
	ErrStreamDisconnected = gocbcore.ErrStreamDisconnected
	// ErrStreamTooSlow occurs when a stream is closed due to being too slow at consuming data.
	ErrStreamTooSlow = gocbcore.ErrStreamTooSlow

	// ErrKeyNotFound occurs when the key is not found on the server.
	ErrKeyNotFound = gocbcore.ErrKeyNotFound
	// ErrKeyExists occurs when the key already exists on the server.
	ErrKeyExists = gocbcore.ErrKeyExists
	// ErrTooBig occurs when the document is too big to be stored.
	ErrTooBig = gocbcore.ErrTooBig
	// ErrNotStored occurs when an item fails to be stored.  Usually an append/prepend to missing key.
	ErrNotStored = gocbcore.ErrNotStored
	// ErrAuthError occurs when there is an issue with authentication (bad password?).
	ErrAuthError = gocbcore.ErrAuthError
	// ErrRangeError occurs when an invalid range is specified.
	ErrRangeError = gocbcore.ErrRangeError
	// ErrRollback occurs when a server rollback has occured making the operation no longer valid.
	ErrRollback = gocbcore.ErrRollback
	// ErrAccessError occurs when you do not have access to the specified resource.
	ErrAccessError = gocbcore.ErrAccessError
	// ErrOutOfMemory occurs when the server has run out of memory to process requests.
	ErrOutOfMemory = gocbcore.ErrOutOfMemory
	// ErrNotSupported occurs when an operation is performed which is not supported.
	ErrNotSupported = gocbcore.ErrNotSupported
	// ErrInternalError occurs when an internal error has prevented an operation from succeeding.
	ErrInternalError = gocbcore.ErrInternalError
	// ErrBusy occurs when the server is too busy to handle your operation.
	ErrBusy = gocbcore.ErrBusy
	// ErrTmpFail occurs when the server is not immediately able to handle your request.
	ErrTmpFail = gocbcore.ErrTmpFail
)
