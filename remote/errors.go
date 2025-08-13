package remote

var (
	// ErrUnAvailable is returned when the remote endpoint is unavailable.
	ErrUnAvailable = &ResponseError{ResponseStatusCodeUNAVAILABLE}
	// ErrTimeout is returned when a remote call times out.
	ErrTimeout = &ResponseError{ResponseStatusCodeTIMEOUT}
	// ErrProcessNameAlreadyExist indicates a process name conflict.
	ErrProcessNameAlreadyExist = &ResponseError{ResponseStatusCodePROCESSNAMEALREADYEXIST}
	// ErrDeadLetter is returned when the target PID cannot be found.
	ErrDeadLetter = &ResponseError{ResponseStatusCodeDeadLetter}
	// ErrUnknownError represents an unspecified remote error.
	ErrUnknownError = &ResponseError{ResponseStatusCodeERROR}
)

// ResponseError is an error type.
// e.g.:
//
//	var err = &ResponseError{1}
type ResponseError struct {
	Code ResponseStatusCode
}

func (r *ResponseError) Error() string {
	if r == nil {
		return "nil"
	}

	return r.Code.String()
}
