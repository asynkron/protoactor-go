package zk

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AsynkronIT/protoactor-go/log"
)

func intToStr(i int) string {
	return strconv.FormatInt(int64(i), 10)
}

func strToInt(s string) int {
	i, _ := strconv.ParseInt(s, 10, 64)
	return int(i)
}

func isStrBlank(s string) bool { return s == "" }

func formatBaseKey(s string) string {
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	if strings.HasSuffix(s, "/") && s != "/" {
		s = strings.TrimSuffix(s, "/")
	}
	return s
}

func parseSeq(path string) (int, error) {
	parts := strings.Split(path, "-")
	if len(parts) == 1 {
		parts = strings.Split(path, "__")
	}
	return strconv.Atoi(parts[len(parts)-1])
}

func stringContains(list []string, str string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

func mapString(list []string, fn func(string) string) []string {
	l := make([]string, len(list))
	for i, str := range list {
		l[i] = fn(str)
	}
	return l
}

func safeRun(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			plog.Warn("OnRoleChanged.", log.Error(fmt.Errorf("%v", r)))
		}
	}()
	fn()
}
