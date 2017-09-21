package remote

type ActorPidRequestStatusCode int32

const (
	ActorPidRequestStatusOK ActorPidRequestStatusCode = iota
	ActorPidRequestStatusUNAVAILABLE
)

func (c ActorPidRequestStatusCode) ToInt32() int32 {
	return int32(c)
}
