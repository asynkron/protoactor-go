package zk

import "github.com/asynkron/protoactor-go/log"

var plog = log.New(log.InfoLevel, "[CLU/ZK]")

// SetLogLevel sets the log level for the logger
// SetLogLevel is safe to be called concurrently
func SetLogLevel(level log.Level) {
	plog.SetLevel(level)
}
