package actor

type messageHeader map[string]string

func (m messageHeader) Get(key string) string {
	return m[key]
}

func (m messageHeader) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

type ReadonlyMessageHeader interface {
	Get(key string) string
	Keys() []string
}

type messageEnvelope struct {
	Header  messageHeader
	Message interface{}
	Sender  *PID
}

var (
	emptyMessageHeader = make(messageHeader)
)
