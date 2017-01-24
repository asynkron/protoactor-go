package log

import (
	"reflect"
	"time"
)

type Encoder interface {
	EncodeBool(key string, val bool)
	EncodeFloat64(key string, val float64)
	EncodeInt(key string, val int)
	EncodeInt64(key string, val int64)
	EncodeDuration(key string, val time.Duration)
	EncodeUint(key string, val uint)
	EncodeUint64(key string, val uint64)
	EncodeString(key string, val string)
	EncodeObject(key string, val interface{})
	EncodeType(key string, val reflect.Type)
}
