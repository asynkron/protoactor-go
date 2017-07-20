/*
Package log provides simple log interfaces
*/
package log

import (
	"sync/atomic"
	"time"
)

type Level int32

const (
	MinLevel = Level(iota)
	DebugLevel
	InfoLevel
	ErrorLevel
	OffLevel
)

type Logger struct {
	level   Level
	prefix  string
	context []Field
}

func New(level Level, prefix string, context ...Field) *Logger {
	return &Logger{level: level, prefix: prefix, context: context}
}

func (l *Logger) With(fields ...Field) *Logger {
	var ctx []Field

	ll := len(l.context) + len(fields)
	if ll > 0 {
		ctx = make([]Field, 0, ll)
		if len(l.context) > 0 {
			ctx = append(ctx, l.context...)
		}

		if len(fields) > 0 {
			ctx = append(ctx, fields...)
		}
	}

	return &Logger{
		level:   l.level,
		prefix:  l.prefix,
		context: ctx,
	}
}

func (l *Logger) Level() Level {
	return Level(atomic.LoadInt32((*int32)(&l.level)))
}

func (l *Logger) SetLevel(level Level) {
	atomic.StoreInt32((*int32)(&l.level), int32(level))
}

func (l *Logger) Debug(msg string, fields ...Field) {
	if l.Level() < InfoLevel {
		es.Publish(Event{Time: time.Now(), Level: DebugLevel, Prefix: l.prefix, Message: msg, Context: l.context, Fields: fields})
	}
}

func (l *Logger) Info(msg string, fields ...Field) {
	if l.Level() < ErrorLevel {
		es.Publish(Event{Time: time.Now(), Level: InfoLevel, Prefix: l.prefix, Message: msg, Context: l.context, Fields: fields})
	}
}

func (l *Logger) Error(msg string, fields ...Field) {
	if l.Level() < OffLevel {
		es.Publish(Event{Time: time.Now(), Level: ErrorLevel, Prefix: l.prefix, Message: msg, Context: l.context, Fields: fields})
	}
}
