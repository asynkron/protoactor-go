/*
Package log provides simple log interfaces
*/
package log

// A Logger is a type that provides basic support for logging messages
type Logger interface {
	// Printf logs a message. Arguments are handled in the manner of fmt.Printf.
	Printf(format string, v ...interface{})

	// Println logs a message. Arguments are handled in the manner of fmt.Println.
	Println(v ...interface{})
}

// DiscardLogger is a logger that discards all log messages
var DiscardLogger Logger = null(0)

type null int

func (null) Printf(format string, v ...interface{}) {}
func (null) Println(v ...interface{})               {}
