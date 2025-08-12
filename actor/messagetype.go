package actor

import "reflect"

// MessageType returns the full type name of the given message, including any
// pointer prefix. It is typically used in logging where the exact Go type is
// desired. If msg is nil, "<nil>" is returned.
func MessageType(msg interface{}) string {
	if msg == nil {
		return "<nil>"
	}
	return reflect.TypeOf(msg).String()
}

// MessageName returns the message type name without a leading pointer prefix.
// This is useful for metrics where stable type names are preferred.
// If msg is nil, "<nil>" is returned.
func MessageName(msg interface{}) string {
	if msg == nil {
		return "<nil>"
	}
	t := reflect.TypeOf(msg)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.String()
}
