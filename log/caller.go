package log

import (
	"runtime"
	"strconv"
	"strings"
)

type CallerInfo struct {
	fname string
	line  int
}

func newCallerInfo(skip int) CallerInfo {
	_, file, no, ok := runtime.Caller(skip)
	if !ok {
		return CallerInfo{"", 0}
	}
	return CallerInfo{fname: file, line: no}
}

func (ci *CallerInfo) ShortFileName() string {
	fname := ci.fname
	idx := strings.LastIndexByte(fname, '/')
	if idx >= len(fname) {
	} else {
		fname = fname[idx+1:]
	}
	return fname
}

func (ci *CallerInfo) String() string {
	return ci.ShortFileName() + ":" + strconv.Itoa(ci.line)
}
