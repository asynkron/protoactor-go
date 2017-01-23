package log

type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

// DiscardLogger is a logger that discards all log messages
var DiscardLogger Logger = null(0)

type null int

func (null) Printf(format string, v ...interface{}) {}
func (null) Println(v ...interface{})               {}
