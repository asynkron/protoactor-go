package remote

type ResponseStatusCode int32

const (
	ResponseStatusCodeOK ResponseStatusCode = iota
	ResponseStatusCodeUNAVAILABLE
	ResponseStatusCodeTIMEOUT
	ResponseStatusCodePROCESSNAMEALREADYEXIST
	ResponseStatusCodeERROR
)

func (c ResponseStatusCode) ToInt32() int32 {
	return int32(c)
}
