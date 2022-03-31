// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package metrics

import "github.com/asynkron/protoactor-go/log"

var plog = log.New(log.DefaultLevel, "[METRICS]")

// SetLogLevel sets the log level for the logger.
//
// SetLogLevel is safe to call concurrently
func SetLogLevel(level log.Level) {
	plog.SetLevel(level)
}
