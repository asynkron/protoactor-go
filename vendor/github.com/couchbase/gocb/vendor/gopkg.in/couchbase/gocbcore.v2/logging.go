package gocbcore

import (
	"fmt"
	"log"
	"os"
)

type LogLevel int

// **VOLATILE** (subject to change)
// Various logging levels (or subsystems) which can categorize the message.
// Currently these are ordered in decreasing severity.
const (
	LogError LogLevel = iota
	LogWarn
	LogInfo
	LogDebug
	LogTrace
	LogSched
	LogMaxVerbosity
)

// **VOLATILE**
// Logging interface. You can either use one of the default loggers
// (DefaultStdioLogger(), VerboseStdioLogger()) or implement your own.
type Logger interface {
	// Outputs logging information:
	// level is the verbosity level
	// offset is the position within the calling stack from which the message
	// originated. This is useful for contextual loggers which retrieve file/line
	// information.
	Log(level LogLevel, offset int, format string, v ...interface{}) error
}

type defaultLogger struct {
	Level    LogLevel
	GoLogger *log.Logger
}

func (l *defaultLogger) Log(level LogLevel, offset int, format string, v ...interface{}) error {
	if level > l.Level {
		return nil
	}
	s := fmt.Sprintf(format, v...)
	l.GoLogger.Output(offset+1, s)
	return nil
}

var (
	globalDefaultLogger = defaultLogger{
		GoLogger: log.New(os.Stderr, "GOCB ", log.Lmicroseconds|log.Lshortfile), Level: LogDebug,
	}

	globalVerboseLogger = defaultLogger{
		GoLogger: globalDefaultLogger.GoLogger, Level: LogMaxVerbosity,
	}

	globalLogger Logger
)

// **DEPRECATED** (Use DefaultStdioLogger() instead)
// Returns the default logger. This actually logs to stderr rather than
// stdout. Use DefaultStdioLogger which has a correct name, since the
// "standard" logger logs to stderr, rather than stdout.
func DefaultStdOutLogger() Logger {
	return &globalDefaultLogger
}

// **UNCOMMITTED**
// Gets the default standard I/O logger. You can then make gocbcore log by using
// the following idiom:
// gocbcore.SetLogger(gocbcore.DefaultStdioLogger())
func DefaultStdioLogger() Logger {
	return &globalDefaultLogger
}

// **UNCOMMITTED**
// This is a more verbose level of DefaultStdioLogger(). Messages pertaining to the
// scheduling of ordinary commands (and their responses) will also be emitted
// gocbcore.SetLogger(gocbcore.VerboseStdioLogger())
func VerboseStdioLogger() Logger {
	return &globalVerboseLogger
}

// **UNCOMMITTED**
// Sets the logger to be used by the library. A logger can be obtained via the
// DefaultStdioLogger() or VerboseStdioLogger() functions. You can also implement
// your own logger using the volatile Logger interface.
func SetLogger(logger Logger) {
	globalLogger = logger
}

func logExf(level LogLevel, offset int, format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.Log(level, offset+1, format, v...)
	}
}

func logDebugf(format string, v ...interface{}) {
	logExf(LogDebug, 2, format, v...)
}

func logSchedf(format string, v ...interface{}) {
	logExf(LogSched, 2, format, v...)
}
