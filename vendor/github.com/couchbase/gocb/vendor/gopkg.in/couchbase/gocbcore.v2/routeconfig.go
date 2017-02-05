package gocbcore

import (
	"crypto/md5"
	"fmt"
	"net"
	"sort"
	"strings"
)

// "Point" in the ring hash entry. See lcbvb_CONTINUUM
type routeKetamaContinuum struct {
	index uint32
	point uint32
}

type routeConfig struct {
	revId        uint
	numReplicas  int
	bktType      BucketType
	kvServerList []string
	capiEpList   []string
	mgmtEpList   []string
	n1qlEpList   []string
	ftsEpList    []string
	vbMap        [][]int
	ketamaMap    []routeKetamaContinuum
}

type KetamaSorter struct {
	elems []routeKetamaContinuum
}

func (c KetamaSorter) Len() int           { return len(c.elems) }
func (c KetamaSorter) Swap(i, j int)      { c.elems[i], c.elems[j] = c.elems[j], c.elems[i] }
func (c KetamaSorter) Less(i, j int) bool { return c.elems[i].point < c.elems[j].point }

func (config *routeConfig) IsValid() bool {
	if len(config.kvServerList) == 0 || len(config.mgmtEpList) == 0 {
		return false
	}
	switch config.bktType {
	case BktTypeCouchbase:
		return len(config.vbMap) > 0 && len(config.vbMap[0]) > 0
	case BktTypeMemcached:
		return len(config.ketamaMap) > 0
	default:
		return false
	}
}

func (config *routeConfig) buildKetama() {
	// Libcouchbase presorts this. Might not strictly be required..
	sort.Strings(config.kvServerList)

	for ss, authority := range config.kvServerList {
		// 160 points per server
		for hh := 0; hh < 40; hh++ {
			hostkey := []byte(fmt.Sprintf("%s-%d", authority, hh))
			digest := md5.Sum(hostkey)

			for nn := 0; nn < 4; nn++ {

				var d1 = uint32(digest[3+nn*4]&0xff) << 24
				var d2 = uint32(digest[2+nn*4]&0xff) << 16
				var d3 = uint32(digest[1+nn*4]&0xff) << 8
				var d4 = uint32(digest[0+nn*4] & 0xff)
				var point = d1 | d2 | d3 | d4

				config.ketamaMap = append(config.ketamaMap, routeKetamaContinuum{
					point: point,
					index: uint32(ss),
				})
			}
		}
	}

	sort.Sort(KetamaSorter{config.ketamaMap})
}

func (config *routeConfig) KetamaHash(key []byte) uint32 {
	digest := md5.Sum(key)

	return ((uint32(digest[3])&0xFF)<<24 |
		(uint32(digest[2])&0xFF)<<16 |
		(uint32(digest[1])&0xFF)<<8 |
		(uint32(digest[0]) & 0xFF)) & 0xffffffff
}

func (config *routeConfig) KetamaNode(hash uint32) uint32 {
	var lowp = uint32(0)
	var highp = uint32(len(config.ketamaMap))
	var maxp = highp

	if len(config.ketamaMap) <= 0 {
		panic("0-length ketama map!")
	}

	// Copied from libcouchbase vbucket.c (map_ketama)
	for {
		midp := lowp + (highp-lowp)/2
		if midp == maxp {
			// Roll over to first entry
			return config.ketamaMap[0].index
		}

		var mid uint32 = config.ketamaMap[midp].point
		var prev uint32
		if midp == 0 {
			prev = 0
		} else {
			prev = config.ketamaMap[midp-1].point
		}

		if hash <= mid && hash > prev {
			return config.ketamaMap[midp].index
		}

		if mid < hash {
			lowp = midp + 1
		} else {
			highp = midp - 1
		}

		if lowp > highp {
			return config.ketamaMap[0].index
		}
	}
}

func buildRouteConfig(bk *cfgBucket, useSsl bool) *routeConfig {
	var kvServerList []string
	var capiEpList []string
	var mgmtEpList []string
	var n1qlEpList []string
	var ftsEpList []string
	var bktType BucketType

	switch bk.NodeLocator {
	case "ketama":
		bktType = BktTypeMemcached
	case "vbucket":
		bktType = BktTypeCouchbase
	default:
		logDebugf("Invalid nodeLocator %s", bk.NodeLocator)
		bktType = BktTypeInvalid
	}

	if bk.NodesExt != nil {
		for _, node := range bk.NodesExt {
			// Hostname blank means to use the same one as was connected to
			if node.Hostname == "" {
				node.Hostname = bk.SourceHostname
			}

			if !useSsl {
				if node.Services.Kv > 0 {
					kvServerList = append(kvServerList, fmt.Sprintf("%s:%d", node.Hostname, node.Services.Kv))
				}
				if node.Services.Capi > 0 {
					capiEpList = append(capiEpList, fmt.Sprintf("http://%s:%d/%s", node.Hostname, node.Services.Capi, bk.Name))
				}
				if node.Services.Mgmt > 0 {
					mgmtEpList = append(mgmtEpList, fmt.Sprintf("http://%s:%d", node.Hostname, node.Services.Mgmt))
				}
				if node.Services.N1ql > 0 {
					n1qlEpList = append(n1qlEpList, fmt.Sprintf("http://%s:%d", node.Hostname, node.Services.N1ql))
				}
				if node.Services.Fts > 0 {
					ftsEpList = append(ftsEpList, fmt.Sprintf("http://%s:%d", node.Hostname, node.Services.Fts))
				}
			} else {
				if node.Services.KvSsl > 0 {
					kvServerList = append(kvServerList, fmt.Sprintf("%s:%d", node.Hostname, node.Services.KvSsl))
				}
				if node.Services.CapiSsl > 0 {
					capiEpList = append(capiEpList, fmt.Sprintf("https://%s:%d/%s", node.Hostname, node.Services.CapiSsl, bk.Name))
				}
				if node.Services.MgmtSsl > 0 {
					mgmtEpList = append(mgmtEpList, fmt.Sprintf("https://%s:%d", node.Hostname, node.Services.MgmtSsl))
				}
				if node.Services.N1qlSsl > 0 {
					n1qlEpList = append(n1qlEpList, fmt.Sprintf("https://%s:%d", node.Hostname, node.Services.N1qlSsl))
				}
				if node.Services.FtsSsl > 0 {
					ftsEpList = append(ftsEpList, fmt.Sprintf("http://%s:%d", node.Hostname, node.Services.FtsSsl))
				}
			}
		}
	} else {
		if useSsl {
			panic("Received config without nodesExt while SSL is enabled.")
		}

		if bktType == BktTypeCouchbase {
			kvServerList = bk.VBucketServerMap.ServerList
		}

		for _, node := range bk.Nodes {
			if node.CouchAPIBase != "" {
				// Slice off the UUID as Go's HTTP client cannot handle being passed URL-Encoded path values.
				capiEp := strings.SplitN(node.CouchAPIBase, "%2B", 2)[0]

				capiEpList = append(capiEpList, capiEp)
			}
			if node.Hostname != "" {
				mgmtEpList = append(mgmtEpList, fmt.Sprintf("http://%s", node.Hostname))
			}

			if bktType == BktTypeMemcached {
				// Get the data port. No VBucketServerMap.
				host, _, err := net.SplitHostPort(node.Hostname)
				if err != nil {
					panic(err)
				}
				curKvHost := fmt.Sprintf("%s:%d", host, node.Ports["direct"])
				kvServerList = append(kvServerList, curKvHost)
			}
		}
	}

	rc := &routeConfig{
		revId:        0,
		kvServerList: kvServerList,
		capiEpList:   capiEpList,
		mgmtEpList:   mgmtEpList,
		n1qlEpList:   n1qlEpList,
		ftsEpList:    ftsEpList,
		vbMap:        bk.VBucketServerMap.VBucketMap,
		numReplicas:  bk.VBucketServerMap.NumReplicas,
		bktType:      bktType,
	}

	if bktType == BktTypeMemcached {
		rc.buildKetama()
	}

	return rc
}
