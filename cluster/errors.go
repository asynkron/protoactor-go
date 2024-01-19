package cluster

import (
	"errors"
	"fmt"
)

const (
	ErrorReason_OK                  = "OK"
	ErrorReason_CANCELLED           = "CANCELLED"
	ErrorReason_UNKNOWN             = "UNKNOWN"
	ErrorReason_INVALID_ARGUMENT    = "INVALID_ARGUMENT"
	ErrorReason_DEADLINE_EXCEEDED   = "DEADLINE_EXCEEDED"
	ErrorReason_NOT_FOUND           = "NOT_FOUND"
	ErrorReason_ALREADY_EXISTS      = "ALREADY_EXISTS"
	ErrorReason_PERMISSION_DENIED   = "PERMISSION_DENIED"
	ErrorReason_RESOURCE_EXHAUSTED  = "RESOURCE_EXHAUSTED"
	ErrorReason_FAILED_PRECONDITION = "FAILED_PRECONDITION"
	ErrorReason_ABORTED             = "ABORTED"
	ErrorReason_OUT_OF_RANGE        = "OUT_OF_RANGE"
	ErrorReason_UNIMPLEMENTED       = "UNIMPLEMENTED"
	ErrorReason_INTERNAL            = "INTERNAL"
	ErrorReason_UNAVAILABLE         = "UNAVAILABLE"
	ErrorReason_DATA_LOSS           = "DATA_LOSS"
	ErrorReason_UNAUTHENTICATED     = "UNAUTHENTICATED"
)

func NewGrainErrorResponse(reason, message string) *GrainErrorResponse {
	return &GrainErrorResponse{
		Reason:  reason,
		Message: message,
	}
}

func NewGrainErrorResponsef(reason, format string, args ...interface{}) *GrainErrorResponse {
	return &GrainErrorResponse{
		Reason:  reason,
		Message: fmt.Sprintf(format, args...),
	}
}

func (m *GrainErrorResponse) Error() string {
	return fmt.Sprintf("grain error response, reason: %s, message: %s, metadata: %v", m.Reason, m.Message, m.Metadata)
}

func (m *GrainErrorResponse) Is(err error) bool {
	if e := new(GrainErrorResponse); errors.As(err, &e) {
		return e.Reason == m.Reason
	}
	return false
}

func (m *GrainErrorResponse) Errorf(format string, args ...interface{}) error {
	return NewGrainErrorResponse(m.Reason, fmt.Sprintf(format, args...))
}

func (m *GrainErrorResponse) WithMetadata(metadata map[string]string) *GrainErrorResponse {
	m.Metadata = metadata
	return m
}

func Reason(err error) string {
	if err == nil {
		return ErrorReason_UNKNOWN
	}
	return FromError(err).Reason
}

func FromError(err error) *GrainErrorResponse {
	if err == nil {
		return nil
	}
	if e := new(GrainErrorResponse); errors.As(err, &e) {
		return e
	}

	ret := NewGrainErrorResponse(
		ErrorReason_UNKNOWN,
		err.Error(),
	)
	return ret
}
