package log

import "time"

type Event struct {
	Time    time.Time
	Level   Level
	Prefix  string
	Caller  CallerInfo
	Message string
	Context []Field
	Fields  []Field
}
