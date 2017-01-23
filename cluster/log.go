package cluster

import (
	"log"
	"os"

	plog "github.com/AsynkronIT/protoactor-go/log"
)

var (
	logerr plog.Logger
	logdbg plog.Logger
)

func init() {
	logerr = log.New(os.Stdout, "[ERROR] [CLUSTER] ", log.Ldate|log.Ltime|log.LUTC)
	logdbg = log.New(os.Stdout, "[DEBUG] [CLUSTER] ", log.Ldate|log.Ltime|log.LUTC)
}

// SetDebugLogger sets the debug logger with an alternate logger
//
// use log.DiscardLogger to discard all log messages
func SetDebugLogger(l plog.Logger) {
	logdbg = l
}

// SetErrorLogger sets the error logger
//
// Error logging is reserved for system errors
// use log.DiscardLogger to discard all log messages
func SetErrorLogger(l plog.Logger) {
	logerr = l
}
