package actor

type messageHeader map[string]string

func (header messageHeader) Get(key string) string {
	return header[key]
}

func (header messageHeader) Set(key string, value string) {
	header[key] = value
}

func (header messageHeader) Keys() []string {
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	return keys
}

func (header messageHeader) Length() int {
	return len(header)
}

func (header messageHeader) ToMap() map[string]string {
	mp := make(map[string]string)
	for k, v := range header {
		mp[k] = v
	}
	return mp
}

// ReadonlyMessageHeader exposes read-only accessors for a message header.
type ReadonlyMessageHeader interface {
	Get(key string) string
	Keys() []string
	Length() int
	ToMap() map[string]string
}

// MessageEnvelope wraps a message along with optional headers and sender.
type MessageEnvelope struct {
	Header  messageHeader
	Message any
	Sender  *PID
}

// GetHeader returns the value of a header key.
func (envelope *MessageEnvelope) GetHeader(key string) string {
	if envelope.Header == nil {
		return ""
	}
	return envelope.Header.Get(key)
}

// SetHeader sets a header key to the given value.
func (envelope *MessageEnvelope) SetHeader(key string, value string) {
	if envelope.Header == nil {
		envelope.Header = make(map[string]string)
	}
	envelope.Header.Set(key, value)
}

// EmptyMessageHeader represents an empty message header.
var EmptyMessageHeader = make(messageHeader)

// WrapEnvelope ensures the message is inside a MessageEnvelope.
func WrapEnvelope(message any) *MessageEnvelope {
	if e, ok := message.(*MessageEnvelope); ok {
		return e
	}
	return &MessageEnvelope{nil, message, nil}
}

// envelopeWithSender returns a *MessageEnvelope carrying message with the
// given sender. If message is already a *MessageEnvelope, its Header is
// preserved; when sender is non-nil the returned envelope is a copy of the
// input with Sender replaced (the caller's envelope is never mutated). If
// sender is nil, an existing envelope is returned as-is.
func envelopeWithSender(message any, sender *PID) *MessageEnvelope {
	if env, ok := message.(*MessageEnvelope); ok {
		if sender == nil {
			return env
		}
		out := *env
		out.Sender = sender
		return &out
	}
	return &MessageEnvelope{Header: nil, Message: message, Sender: sender}
}

// EnvelopeWithHeaders returns a *MessageEnvelope carrying message with the
// given headers attached. If message is already a *MessageEnvelope, the
// returned envelope is a copy: existing envelope header values win on key
// conflict (explicit wrap is more specific than caller-provided headers).
// The caller's envelope is never mutated. If headers is empty, an existing
// envelope is returned as-is and a raw message is wrapped without a header
// map.
func EnvelopeWithHeaders(message any, headers map[string]string) *MessageEnvelope {
	if env, ok := message.(*MessageEnvelope); ok {
		if len(headers) == 0 {
			return env
		}
		out := &MessageEnvelope{
			Header:  make(messageHeader, len(headers)+env.Header.Length()),
			Message: env.Message,
			Sender:  env.Sender,
		}
		for k, v := range headers {
			out.Header[k] = v
		}
		for _, k := range env.Header.Keys() {
			out.Header[k] = env.Header.Get(k) // envelope wins
		}
		return out
	}
	out := &MessageEnvelope{Message: message}
	if len(headers) > 0 {
		out.Header = make(messageHeader, len(headers))
		for k, v := range headers {
			out.Header[k] = v
		}
	}
	return out
}

// UnwrapEnvelope extracts header, message and sender from an envelope.
func UnwrapEnvelope(message any) (ReadonlyMessageHeader, any, *PID) {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Header, env.Message, env.Sender
	}
	return nil, message, nil
}

// UnwrapEnvelopeHeader returns the header from an envelope.
func UnwrapEnvelopeHeader(message any) ReadonlyMessageHeader {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Header
	}
	return nil
}

// UnwrapEnvelopeMessage returns the message from an envelope.
func UnwrapEnvelopeMessage(message any) any {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Message
	}
	return message
}

// UnwrapEnvelopeSender returns the sender from an envelope.
func UnwrapEnvelopeSender(message any) *PID {
	if env, ok := message.(*MessageEnvelope); ok {
		return env.Sender
	}
	return nil
}
