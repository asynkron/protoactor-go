package remote

var (
	ErrUnAvailable             = &ResponseError{ResponseStatusCodeUNAVAILABLE}
	ErrTimeout                 = &ResponseError{ResponseStatusCodeTIMEOUT}
	ErrProcessNameAlreadyExist = &ResponseError{ResponseStatusCodePROCESSNAMEALREADYEXIST}
	ErrDeadLetter              = &ResponseError{ResponseStatusCodeDeadLetter}
	ErrUnknownError            = &ResponseError{ResponseStatusCodeERROR}
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
