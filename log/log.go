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
)

type Logger struct {
	level   Level
	prefix  string
	context []Field
}

func New(level Level, prefix string, context ...Field) *Logger {
	return &Logger{level: level, prefix: prefix, context: context}
}

func (l *Logger) Level() Level {
	return Level(atomic.LoadInt32((*int32)(&l.level)))
}

func (l *Logger) SetLevel(level Level) {
	atomic.StoreInt32((*int32)(&l.level), int32(level))
}

func (l *Logger) Debug(msg string, fields ...Field) {
	if l.Level() > MinLevel {
		es.Publish(Event{Time: time.Now(), Level: DebugLevel, Prefix: l.prefix, Message: msg, Context: l.context, Fields: fields})
	}
}

func (l *Logger) Info(msg string, fields ...Field) {
	if l.Level() > DebugLevel {
		es.Publish(Event{Time: time.Now(), Level: DebugLevel, Prefix: l.prefix, Message: msg, Context: l.context, Fields: fields})
	}
}

func (l *Logger) Error(msg string, fields ...Field) {
	if l.Level() > InfoLevel {
		es.Publish(Event{Time: time.Now(), Level: DebugLevel, Prefix: l.prefix, Message: msg, Context: l.context, Fields: fields})
	}
}
