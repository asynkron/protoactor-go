package gocbcore

import (
	"errors"
	"fmt"
)

type SubDocMutateError struct {
	Err     error
	OpIndex int
}

func (e SubDocMutateError) Error() string {
	return fmt.Sprintf("Subdocument mutation %d failed (%s)", e.OpIndex, e.Err.Error())
}

type timeoutError struct {
}

func (e timeoutError) Error() string {
	return "The operation has timed out."
}
func (e timeoutError) Timeout() bool {
	return true
}

type networkError struct {
}

func (e networkError) Error() string {
	return "Network error."
}

// Included for legacy support.
func (e networkError) NetworkError() bool {
	return true
}

type overloadError struct {
}

func (e overloadError) Error() string {
	return "Queue overflow."
}
func (e overloadError) Overload() bool {
	return true
}

type shutdownError struct {
}

func (e shutdownError) Error() string {
	return "Connection shut down."
}

// Legacy
func (e shutdownError) ShutdownError() bool {
	return true
}

type memdError struct {
	code StatusCode
}

func (e memdError) Error() string {
	switch e.code {
	case StatusSuccess:
		return "Success."
	case StatusKeyNotFound:
		return "Key not found."
	case StatusKeyExists:
		return "Key already exists."
	case StatusTooBig:
		return "Document value was too large."
	case StatusNotStored:
		return "The document could not be stored."
	case StatusBadDelta:
		return "An invalid delta was passed."
	case StatusNotMyVBucket:
		return "Operation sent to incorrect server."
	case StatusNoBucket:
		return "Not connected to a bucket."
	case StatusAuthStale:
		return "The authenication context is stale. Try re-authenticating."
	case StatusAuthError:
		return "Authentication Error."
	case StatusAuthContinue:
		return "Auth Continue."
	case StatusRangeError:
		return "Requested value is outside range."
	case StatusAccessError:
		return "No access."
	case StatusNotInitialized:
		return "The cluster is being initialized. Requests are blocked."
	case StatusRollback:
		return "A rollback is required."
	case StatusUnknownCommand:
		return "An unknown command was received."
	case StatusOutOfMemory:
		return "The server is out of memory."
	case StatusNotSupported:
		return "The server does not support this command."
	case StatusInternalError:
		return "Internal server error."
	case StatusBusy:
		return "The server is busy. Try again later."
	case StatusTmpFail:
		return "A temporary failure occurred.  Try again later."
	case StatusSubDocPathNotFound:
		return "Sub-document path does not exist"
	case StatusSubDocPathMismatch:
		return "Type of element in sub-document path conflicts with type in document."
	case StatusSubDocPathInvalid:
		return "Malformed sub-document path."
	case StatusSubDocPathTooBig:
		return "Sub-document contains too many components."
	case StatusSubDocDocTooDeep:
		return "Existing document contains too many levels of nesting."
	case StatusSubDocCantInsert:
		return "Subdocument operation would invalidate the JSON."
	case StatusSubDocNotJson:
		return "Existing document is not valid JSON."
	case StatusSubDocBadRange:
		return "The existing numeric value is too large."
	case StatusSubDocBadDelta:
		return "The numeric operation would yield a number that is too large, or " +
			"a zero delta was specified."
	case StatusSubDocPathExists:
		return "The given path already exists in the document."
	case StatusSubDocValueTooDeep:
		return "Value is too deep to insert."
	case StatusSubDocBadCombo:
		return "Incorrectly matched subdocument operation types."
	case StatusSubDocBadMulti:
		return "Could not execute one or more multi lookups or mutations."
	default:
		return fmt.Sprintf("An unknown error occurred (%d).", e.code)
	}
}
func (e memdError) Temporary() bool {
	return e.code == StatusOutOfMemory || e.code == StatusTmpFail || e.code == StatusBusy
}

/* Legacy MemdError Handlers */
func (e memdError) Success() bool {
	return e.code == StatusSuccess
}
func (e memdError) KeyNotFound() bool {
	return e.code == StatusKeyNotFound
}
func (e memdError) KeyExists() bool {
	return e.code == StatusKeyExists
}
func (e memdError) AuthStale() bool {
	return e.code == StatusAuthStale
}
func (e memdError) AuthError() bool {
	return e.code == StatusAuthError
}
func (e memdError) AuthContinue() bool {
	return e.code == StatusAuthContinue
}
func (e memdError) ValueTooBig() bool {
	return e.code == StatusTooBig
}
func (e memdError) NotStored() bool {
	return e.code == StatusNotStored
}
func (e memdError) BadDelta() bool {
	return e.code == StatusBadDelta
}
func (e memdError) NotMyVBucket() bool {
	return e.code == StatusNotMyVBucket
}
func (e memdError) NoBucket() bool {
	return e.code == StatusNoBucket
}
func (e memdError) RangeError() bool {
	return e.code == StatusRangeError
}
func (e memdError) AccessError() bool {
	return e.code == StatusAccessError
}
func (e memdError) NotIntializedError() bool {
	return e.code == StatusNotInitialized
}
func (e memdError) Rollback() bool {
	return e.code == StatusRollback
}
func (e memdError) UnknownCommandError() bool {
	return e.code == StatusUnknownCommand
}
func (e memdError) NotSupportedError() bool {
	return e.code == StatusNotSupported
}
func (e memdError) InternalError() bool {
	return e.code == StatusInternalError
}
func (e memdError) BusyError() bool {
	return e.code == StatusBusy
}

type streamEndError struct {
	code StreamEndStatus
}

func (e streamEndError) Error() string {
	switch e.code {
	case StreamEndOK:
		return "Success."
	case StreamEndClosed:
		return "Stream closed."
	case StreamEndStateChanged:
		return "State changed."
	case StreamEndDisconnected:
		return "Disconnected."
	case StreamEndTooSlow:
		return "Too slow."
	default:
		return fmt.Sprintf("Stream closed for unknown reason: (%d).", e.code)
	}
}

func (e streamEndError) Success() bool {
	return e.code == StreamEndOK
}
func (e streamEndError) Closed() bool {
	return e.code == StreamEndClosed
}
func (e streamEndError) StateChanged() bool {
	return e.code == StreamEndStateChanged
}
func (e streamEndError) Disconnected() bool {
	return e.code == StreamEndDisconnected
}
func (e streamEndError) TooSlow() bool {
	return e.code == StreamEndTooSlow
}

var (
	ErrDispatchFail   = errors.New("Failed to dispatch operation.")
	ErrBadHosts       = errors.New("Failed to connect to any of the specified hosts.")
	ErrProtocol       = errors.New("Failed to parse server response.")
	ErrNoReplicas     = errors.New("No replicas responded in time.")
	ErrInvalidServer  = errors.New("The specific server index is invalid.")
	ErrInvalidVBucket = errors.New("The specific vbucket index is invalid.")
	ErrInvalidReplica = errors.New("The specific server index is invalid.")

	ErrInvalidCert = errors.New("The certificate is invalid.")

	ErrShutdown = &shutdownError{}
	ErrOverload = &overloadError{}
	ErrNetwork  = &networkError{}
	ErrTimeout  = &timeoutError{}

	ErrStreamClosed       = &streamEndError{StreamEndClosed}
	ErrStreamStateChanged = &streamEndError{StreamEndStateChanged}
	ErrStreamDisconnected = &streamEndError{StreamEndDisconnected}
	ErrStreamTooSlow      = &streamEndError{StreamEndTooSlow}

	ErrKeyNotFound        = &memdError{StatusKeyNotFound}
	ErrKeyExists          = &memdError{StatusKeyExists}
	ErrTooBig             = &memdError{StatusTooBig}
	ErrInvalidArgs        = &memdError{StatusInvalidArgs}
	ErrNotStored          = &memdError{StatusNotStored}
	ErrBadDelta           = &memdError{StatusBadDelta}
	ErrNotMyVBucket       = &memdError{StatusNotMyVBucket}
	ErrNoBucket           = &memdError{StatusNoBucket}
	ErrAuthStale          = &memdError{StatusAuthStale}
	ErrAuthError          = &memdError{StatusAuthError}
	ErrAuthContinue       = &memdError{StatusAuthContinue}
	ErrRangeError         = &memdError{StatusRangeError}
	ErrRollback           = &memdError{StatusRollback}
	ErrAccessError        = &memdError{StatusAccessError}
	ErrNotInitialized     = &memdError{StatusNotInitialized}
	ErrUnknownCommand     = &memdError{StatusUnknownCommand}
	ErrOutOfMemory        = &memdError{StatusOutOfMemory}
	ErrNotSupported       = &memdError{StatusNotSupported}
	ErrInternalError      = &memdError{StatusInternalError}
	ErrBusy               = &memdError{StatusBusy}
	ErrTmpFail            = &memdError{StatusTmpFail}
	ErrSubDocPathNotFound = &memdError{StatusSubDocPathNotFound}
	ErrSubDocPathMismatch = &memdError{StatusSubDocPathMismatch}
	ErrSubDocPathInvalid  = &memdError{StatusSubDocPathInvalid}
	ErrSubDocPathTooBig   = &memdError{StatusSubDocPathTooBig}
	ErrSubDocDocTooDeep   = &memdError{StatusSubDocDocTooDeep}
	ErrSubDocCantInsert   = &memdError{StatusSubDocCantInsert}
	ErrSubDocNotJson      = &memdError{StatusSubDocNotJson}
	ErrSubDocBadRange     = &memdError{StatusSubDocBadRange}
	ErrSubDocBadDelta     = &memdError{StatusSubDocBadDelta}
	ErrSubDocPathExists   = &memdError{StatusSubDocPathExists}
	ErrSubDocValueTooDeep = &memdError{StatusSubDocValueTooDeep}
	ErrSubDocBadCombo     = &memdError{StatusSubDocBadCombo}
	ErrSubDocBadMulti     = &memdError{StatusSubDocBadMulti}
)

func getStreamEndError(code StreamEndStatus) error {
	switch code {
	case StreamEndOK:
		return nil
	case StreamEndClosed:
		return ErrStreamClosed
	case StreamEndStateChanged:
		return ErrStreamStateChanged
	case StreamEndDisconnected:
		return ErrStreamDisconnected
	case StreamEndTooSlow:
		return ErrStreamTooSlow
	default:
		return &streamEndError{code}
	}
}

func getMemdError(code StatusCode) error {
	switch code {
	case StatusSuccess:
		return nil
	case StatusKeyNotFound:
		return ErrKeyNotFound
	case StatusKeyExists:
		return ErrKeyExists
	case StatusTooBig:
		return ErrTooBig
	case StatusInvalidArgs:
		return ErrInvalidArgs
	case StatusNotStored:
		return ErrNotStored
	case StatusBadDelta:
		return ErrBadDelta
	case StatusNotMyVBucket:
		return ErrNotMyVBucket
	case StatusNoBucket:
		return ErrNoBucket
	case StatusAuthStale:
		return ErrAuthStale
	case StatusAuthError:
		return ErrAuthError
	case StatusAuthContinue:
		return ErrAuthContinue
	case StatusRangeError:
		return ErrRangeError
	case StatusAccessError:
		return ErrAccessError
	case StatusNotInitialized:
		return ErrNotInitialized
	case StatusRollback:
		return ErrRollback
	case StatusUnknownCommand:
		return ErrUnknownCommand
	case StatusOutOfMemory:
		return ErrOutOfMemory
	case StatusNotSupported:
		return ErrNotSupported
	case StatusInternalError:
		return ErrInternalError
	case StatusBusy:
		return ErrBusy
	case StatusTmpFail:
		return ErrTmpFail
	case StatusSubDocPathNotFound:
		return ErrSubDocPathNotFound
	case StatusSubDocPathMismatch:
		return ErrSubDocPathMismatch
	case StatusSubDocPathInvalid:
		return ErrSubDocPathInvalid
	case StatusSubDocPathTooBig:
		return ErrSubDocPathTooBig
	case StatusSubDocDocTooDeep:
		return ErrSubDocDocTooDeep
	case StatusSubDocCantInsert:
		return ErrSubDocCantInsert
	case StatusSubDocNotJson:
		return ErrSubDocNotJson
	case StatusSubDocBadRange:
		return ErrSubDocBadRange
	case StatusSubDocBadDelta:
		return ErrSubDocBadDelta
	case StatusSubDocPathExists:
		return ErrSubDocPathExists
	case StatusSubDocValueTooDeep:
		return ErrSubDocValueTooDeep
	case StatusSubDocBadCombo:
		return ErrSubDocBadCombo
	case StatusSubDocBadMulti:
		return ErrSubDocBadMulti
	default:
		return &memdError{code}
	}
}
