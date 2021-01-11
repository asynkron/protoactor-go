package log

import (
	"os"
)

var (
	Development = &Options{
		logLevel:     DebugLevel,
		enableCaller: true,
	}

	Production = &Options{
		logLevel:     InfoLevel,
		enableCaller: false,
	}

	Current = Production
)

func init() {
	env := os.Getenv("PROTO_ACTOR_ENV")
	switch env {
	case "dev":
		Current = Development
	case "prod":
		Current = Production
	default:
		Current = Production
	}
}

// Options for log.
type Options struct {
	logLevel     Level
	enableCaller bool
}

// Setup is used to configure the log system
func (o *Options) With(opts ...option) *Options {
	cloned := *o
	for _, opt := range opts {
		opt(&cloned)
	}
	return &cloned
}

type option func(*Options)

// WithEventSubscriber option replaces the default Event subscriber with fn.
//
// Specifying nil will disable logging of events.
func WithEventSubscriber(fn func(evt Event)) option {
	return func(opts *Options) {
		resetEventSubscriber(fn)
	}
}

// WithCaller option will print the file name and line number.
func WithCaller(enabled bool) option {
	return func(opts *Options) {
		opts.enableCaller = enabled
	}
}

func WithDefaultLevel(level Level) option {
	if level == DefaultLevel {
		level = InfoLevel
	}
	return func(opts *Options) {
		opts.logLevel = level
	}
}

func SetOptions(opts ...option) {
	Current = Current.With(opts...)
}
