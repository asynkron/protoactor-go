package consul

import "github.com/AsynkronIT/protoactor-go/log"

var (
	plog = log.New(log.DebugLevel, "[CLUSTER] [CONSUL]")
)

// SetLogLevel sets the log level for the logger
// SetLogLevel is safe to be called concurrently
func SetLogLevel(level log.Level) {
	plog.SetLevel(level)
}
