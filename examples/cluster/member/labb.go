package main

import (
	"log"
	"math"
)

func main() {
	var h uint32 = 3272168967

	members := []uint32{1972499873, 4155391341}
	bestV := uint32(math.MaxUint32)
	bestI := -1

	//walk all members and find the node with the closest distance to the id hash
	for i, n := range members {
		if b := delta(n, h); b < bestV { //this is not correct. it matches badly
			log.Printf("hash is closest to %v", n)
			bestV = b
			bestI = i
		}
	}

	member := members[bestI]
	log.Printf("[CLUSTER] matched best %v", member)
}

func delta(l uint32, r uint32) uint32 {
	if l > r {
		return l - r
	}
	return r - l
}
