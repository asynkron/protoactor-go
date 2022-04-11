/*
Package log provides simple log interfaces
*/
package log

import (
	"sync/atomic"
	"time"
)

// Level of log.
type Level int32

const (
	MinLevel = Level(iota)
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	OffLevel
	DefaultLevel
)

var levelNames = [OffLevel + 1]string{"-    ", "DEBUG", "INFO ", "WARN", "ERROR", "-    "}

func (l Level) String() string {
	return levelNames[int(l)]
}

type Logger struct {
	level        Level
	prefix       string
	context      []Field
	enableCaller bool
}

// New a Logger
func New(level Level, prefix string, context ...Field) *Logger {
	opts := Current
	if level == DefaultLevel {
		level = opts.logLevel
	}
	return &Logger{
		level:        level,
		prefix:       prefix,
		context:      context,
		enableCaller: opts.enableCaller,
	}
}

func (l *Logger) WithCaller() *Logger {
	l.enableCaller = true
	return l
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

func (l *Logger) newEvent(msg string, level Level, fields ...Field) Event {
	ev := Event{
		Time:    time.Now(),
		Level:   level,
		Prefix:  l.prefix,
		Message: msg,
		Context: l.context,
		Fields:  fields,
	}
	if l.enableCaller {
		ev.Caller = newCallerInfo(3)
	}
	return ev
}

func (l *Logger) Debug(msg string, fields ...Field) {
	if l.Level() <= DebugLevel {
		es.Publish(l.newEvent(msg, DebugLevel, fields...))
	}
}

func (l *Logger) Info(msg string, fields ...Field) {
	if l.Level() <= InfoLevel {
		es.Publish(l.newEvent(msg, InfoLevel, fields...))
	}
}

func (l *Logger) Warn(msg string, fields ...Field) {
	if l.Level() <= WarnLevel {
		es.Publish(l.newEvent(msg, WarnLevel, fields...))
	}
}

func (l *Logger) Error(msg string, fields ...Field) {
	if l.Level() <= ErrorLevel {
		es.Publish(l.newEvent(msg, ErrorLevel, fields...))
	}
}
