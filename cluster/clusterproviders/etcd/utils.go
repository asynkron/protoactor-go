package etcd

import (
	"fmt"
	"strings"
)

func getNodeID(key string, sep string) (string, error) {
	tmpArr := strings.Split(key, sep)
	if len(tmpArr) == 0 {
		return "", fmt.Errorf("invalid key or sep")
	}
	lastIndex := len(tmpArr) - 1
	return tmpArr[lastIndex], nil
}
