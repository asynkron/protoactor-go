package log

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"
	"time"
)

type fieldType int

const (
	unknownType fieldType = iota
	boolType
	floatType
	intType
	int64Type
	durationType
	uintType
	uint64Type
	stringType
	stringerType
	errorType
	objectType
	typeOfType
	skipType
)

type Field struct {
	key       string
	fieldType fieldType
	val       int64
	str       string
	obj       interface{}
}

// Bool constructs a Field with the given key and value.
func Bool(key string, val bool) Field {
	var ival int64
	if val {
		ival = 1
	}

	return Field{key: key, fieldType: boolType, val: ival}
}

// Float64 constructs a Field with the given key and value.
func Float64(key string, val float64) Field {
	return Field{key: key, fieldType: floatType, val: int64(math.Float64bits(val))}
}

// Int constructs a Field with the given key and value. Marshaling ints is lazy.
func Int(key string, val int) Field {
	return Field{key: key, fieldType: intType, val: int64(val)}
}

// Int64 constructs a Field with the given key and value.
func Int64(key string, val int64) Field {
	return Field{key: key, fieldType: int64Type, val: val}
}

// Uint constructs a Field with the given key and value.
func Uint(key string, val uint) Field {
	return Field{key: key, fieldType: uintType, val: int64(val)}
}

// Uint64 constructs a Field with the given key and value.
func Uint64(key string, val uint64) Field {
	return Field{key: key, fieldType: uint64Type, val: int64(val)}
}

// String constructs a Field with the given key and value.
func String(key string, val string) Field {
	return Field{key: key, fieldType: stringType, str: val}
}

// Stringer constructs a Field with the given key and the output of the value's
// String method. The String is not evaluated until encoding.
func Stringer(key string, val fmt.Stringer) Field {
	if val == nil {
		return Field{key: key, fieldType: objectType, obj: val}
	}
	return Field{key: key, fieldType: stringerType, obj: val}
}

// Time constructs a Field with the given key and value. It represents a
// time.Time as a floating-point number of seconds since the Unix epoch.
func Time(key string, val time.Time) Field {
	return Float64(key, float64(val.UnixNano())/float64(time.Second))
}

// Error constructs a Field that lazily stores err.Error() under the key
// "error". If passed a nil error, the field is skipped.
func Error(err error) Field {
	if err == nil {
		return Field{fieldType: skipType}
	}
	return Field{key: "error", fieldType: errorType, obj: err}
}

// Stack constructs a Field that stores a stacktrace under the key "stacktrace".
//
// This is eager and therefore an expensive operation.
func Stack() Field {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(4, pc[:])
	callers := pc[:n]
	frames := runtime.CallersFrames(callers)
	for {
		frame, more := frames.Next()
		file = frame.File
		line = frame.Line
		name = frame.Function
		if !strings.HasPrefix(name, "runtime.") || !more {
			break
		}
	}

	var str string
	switch {
	case name != "":
		str = fmt.Sprintf("%v:%v", name, line)
	case file != "":
		str = fmt.Sprintf("%v:%v", file, line)
	default:
		str = fmt.Sprintf("pc:%x", pc)
	}
	return String("stacktrace", str)
}

// Duration constructs a Field with the given key and value.
func Duration(key string, val time.Duration) Field {
	return Field{key: key, fieldType: durationType, val: int64(val)}
}

// Object constructs a field with the given key and an arbitrary object.
func Object(key string, val interface{}) Field {
	return Field{key: key, fieldType: objectType, obj: val}
}

// TypeOf constructs a field with the given key and an arbitrary object that will log the type information lazily.
func TypeOf(key string, val interface{}) Field {
	return Field{key: key, fieldType: typeOfType, obj: val}
}

// Message constructs a field to store the message under the key message
func Message(val interface{}) Field {
	return Field{key: "message", fieldType: objectType, obj: val}
}

// Encode encodes a field to a type safe val via the encoder.
func (f Field) Encode(enc Encoder) {
	switch f.fieldType {
	case boolType:
		enc.EncodeBool(f.key, f.val == 1)
	case floatType:
		enc.EncodeFloat64(f.key, math.Float64frombits(uint64(f.val)))
	case intType:
		enc.EncodeInt(f.key, int(f.val))
	case int64Type:
		enc.EncodeInt64(f.key, f.val)
	case durationType:
		enc.EncodeDuration(f.key, time.Duration(f.val))
	case uintType:
		enc.EncodeUint(f.key, uint(f.val))
	case uint64Type:
		enc.EncodeUint64(f.key, uint64(f.val))
	case stringType:
		enc.EncodeString(f.key, f.str)
	case stringerType:
		enc.EncodeString(f.key, f.obj.(fmt.Stringer).String())
	case errorType:
		enc.EncodeString(f.key, f.obj.(error).Error())
	case objectType:
		enc.EncodeObject(f.key, f.obj)
	case typeOfType:
		enc.EncodeType(f.key, reflect.TypeOf(f.obj))
	case skipType:
		break
	default:
		panic(fmt.Sprintf("unknown field type found: %v", f))
	}
}
