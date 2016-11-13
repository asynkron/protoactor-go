package cluster

import "hash/fnv"

func delta(l uint32, r uint32) uint32 {
	if l > r {
		return l - r
	}
	return r - l
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
