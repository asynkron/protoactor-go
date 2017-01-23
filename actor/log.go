package actor

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
	logerr = log.New(os.Stdout, "[ERROR] [ACTOR] ", log.Ldate|log.Ltime|log.LUTC)
	logdbg = log.New(os.Stdout, "[DEBUG] [ACTOR] ", log.Ldate|log.Ltime|log.LUTC)
}

// SetDebugLogger sets the debug logger
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
