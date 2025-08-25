package actor

import (
	"reflect"
	"sync"
)

// messageTypeCache stores string representations for message types keyed by
// reflect.Type. It avoids repeated reflection-based string conversions.
var messageTypeCache sync.Map // map[reflect.Type]cachedMessageType

type cachedMessageType struct {
	full string
	name string
}

// MessageType returns the full type name of the given message, including any
// pointer prefix. It is typically used in logging where the exact Go type is
// desired. If msg is nil, "<nil>" is returned.
func MessageType(msg interface{}) string {
	if msg == nil {
		return "<nil>"
	}
	t := reflect.TypeOf(msg)
	if v, ok := messageTypeCache.Load(t); ok {
		return v.(cachedMessageType).full
	}
	c := cacheMessageType(t)
	return c.full
}

// MessageName returns the message type name without a leading pointer prefix.
// This is useful for metrics where stable type names are preferred.
// If msg is nil, "<nil>" is returned.
func MessageName(msg interface{}) string {
	if msg == nil {
		return "<nil>"
	}
	t := reflect.TypeOf(msg)
	if v, ok := messageTypeCache.Load(t); ok {
		return v.(cachedMessageType).name
	}
	c := cacheMessageType(t)
	return c.name
}

// cacheMessageType computes and stores string representations for a reflect.Type.
func cacheMessageType(t reflect.Type) cachedMessageType {
	c := cachedMessageType{full: t.String(), name: t.String()}
	if t.Kind() == reflect.Ptr {
		c.name = t.Elem().String()
	}
	messageTypeCache.Store(t, c)
	return c
}
