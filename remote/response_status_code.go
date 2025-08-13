package remote

import "strconv"

// ResponseStatusCode represents possible outcomes of remote operations.
type ResponseStatusCode int32

const (
	// ResponseStatusCodeOK indicates the operation completed successfully.
	ResponseStatusCodeOK ResponseStatusCode = iota
	// ResponseStatusCodeUNAVAILABLE indicates the remote endpoint was unavailable.
	ResponseStatusCodeUNAVAILABLE
	// ResponseStatusCodeTIMEOUT indicates the operation timed out.
	ResponseStatusCodeTIMEOUT
	// ResponseStatusCodePROCESSNAMEALREADYEXIST indicates a process name conflict.
	ResponseStatusCodePROCESSNAMEALREADYEXIST
	// ResponseStatusCodeERROR indicates an unspecified error occurred.
	ResponseStatusCodeERROR
	// ResponseStatusCodeDeadLetter indicates the target PID could not be found.
	ResponseStatusCodeDeadLetter
	// ResponseStatusCodeMAX is a boundary marker for the enum.
	ResponseStatusCodeMAX // just a boundary.
)

var responseNames [ResponseStatusCodeMAX]string

func init() {
	responseNames[ResponseStatusCodeOK] = "ResponseStatusCodeOK"
	responseNames[ResponseStatusCodeUNAVAILABLE] = "ResponseStatusCodeUNAVAILABLE"
	responseNames[ResponseStatusCodeTIMEOUT] = "ResponseStatusCodeTIMEOUT"
	responseNames[ResponseStatusCodePROCESSNAMEALREADYEXIST] = "ResponseStatusCodePROCESSNAMEALREADYEXIST"
	responseNames[ResponseStatusCodePROCESSNAMEALREADYEXIST] = "ResponseStatusCodePROCESSNAMEALREADYEXIST"
	responseNames[ResponseStatusCodeERROR] = "ResponseStatusCodeERROR"
	responseNames[ResponseStatusCodeDeadLetter] = "ResponseStatusCodeDeadLetter"
}

// ToInt32 converts the status code to its int32 representation.
func (c ResponseStatusCode) ToInt32() int32 {
	return int32(c)
}

func (c ResponseStatusCode) String() string {
	statusCode := int(c)
	if statusCode < 0 || statusCode >= len(responseNames) {
		return "ResponseStatusCode-" + strconv.Itoa(int(c))
	}
	return responseNames[statusCode]
}

// AsError converts the status code to a ResponseError.
func (c ResponseStatusCode) AsError() *ResponseError {
	switch c {
	case ResponseStatusCodeOK:
		return nil
	case ResponseStatusCodeUNAVAILABLE:
		return ErrUnAvailable
	case ResponseStatusCodeTIMEOUT:
		return ErrTimeout
	case ResponseStatusCodePROCESSNAMEALREADYEXIST:
		return ErrProcessNameAlreadyExist
	case ResponseStatusCodeERROR:
		return ErrUnknownError
	case ResponseStatusCodeDeadLetter:
		return ErrDeadLetter
	default:
		return &ResponseError{c}
	}
}
