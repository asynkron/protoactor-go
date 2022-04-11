package remote

import "strconv"

type ResponseStatusCode int32

const (
	ResponseStatusCodeOK ResponseStatusCode = iota
	ResponseStatusCodeUNAVAILABLE
	ResponseStatusCodeTIMEOUT
	ResponseStatusCodePROCESSNAMEALREADYEXIST
	ResponseStatusCodeERROR
	ResponseStatusCodeDeadLetter
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
