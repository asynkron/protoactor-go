package actor

import (
	slog "log"
	"os"

	"github.com/AsynkronIT/protoactor-go/log"
)

var (
	logerr log.Logger
	logdbg log.Logger
)

func init() {
	logerr = slog.New(os.Stdout, "[ERROR] [ACTOR] ", slog.Ldate|slog.Ltime|slog.LUTC)
	logdbg = slog.New(os.Stdout, "[DEBUG] [ACTOR] ", slog.Ldate|slog.Ltime|slog.LUTC)
}

// SetDebugLogger sets the debug logger.
//
// Use log.DiscardLogger to discard all log messages.
//
// SetDebugLogger is not safe to call concurrently and must be configured before starting the actor system
func SetDebugLogger(l log.Logger) {
	logdbg = l
}

// SetErrorLogger sets the error logger.
//
// Error logging is reserved for system errors.
// Use log.DiscardLogger to discard all log messages.
//
// SetErrorLogger is not safe to call concurrently and must be configured before starting the actor system
func SetErrorLogger(l log.Logger) {
	logerr = l
}
