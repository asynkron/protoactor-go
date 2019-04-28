package actor

type messageHeader map[string]string

func (m messageHeader) Get(key string) string {
	return m[key]
}

func (m messageHeader) Set(key string, value string) {
	m[key] = value
}

func (m messageHeader) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m messageHeader) Length() int {
	return len(m)
}

func (m messageHeader) ToMap() map[string]string {
	mp := make(map[string]string)
	for k, v := range m {
		mp[k] = v
	}
	return mp
}

type ReadonlyMessageHeader interface {
	Get(key string) string
	Keys() []string
	Length() int
	ToMap() map[string]string
}

type MessageEnvelope struct {
	Header  messageHeader
	Message interface{}
	Sender  *PID
}

func (me *MessageEnvelope) GetHeader(key string) string {
	if me.Header == nil {
		return ""
	}
	return me.Header.Get(key)
}

func (me *MessageEnvelope) SetHeader(key string, value string) {
	if me.Header == nil {
		me.Header = make(map[string]string)
	}
	me.Header.Set(key, value)
}

var (
	EmptyMessageHeader = make(messageHeader)
)

func WrapEnvelope(message interface{}) *MessageEnvelope {
	if e, ok := message.(*MessageEnvelope); ok {
		return e
	}
	return &MessageEnvelope{nil, message, nil}
}

func UnwrapEnvelope(message interface{}) (ReadonlyMessageHeader, interface{}, *PID) {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Header, env.Message, env.Sender
	}
	return nil, message, nil
}

func UnwrapEnvelopeHeader(message interface{}) ReadonlyMessageHeader {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Header
	}
	return nil
}

func UnwrapEnvelopeMessage(message interface{}) interface{} {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Message
	}
	return message
}

func UnwrapEnvelopeSender(message interface{}) *PID {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Sender
	}
	return nil
}
