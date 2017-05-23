package gocbcore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type configStreamBlock struct {
	Bytes []byte
}

func (i *configStreamBlock) UnmarshalJSON(data []byte) error {
	i.Bytes = make([]byte, len(data))
	copy(i.Bytes, data)
	return nil
}

func hostnameFromUri(uri string) string {
	uriInfo, err := url.Parse(uri)
	if err != nil {
		panic("Failed to parse URI to hostname!")
	}
	return strings.Split(uriInfo.Host, ":")[0]
}

func (c *Agent) httpLooper(firstCfgFn func(*cfgBucket, error) bool) {
	waitPeriod := 20 * time.Second
	maxConnPeriod := 10 * time.Second
	var iterNum uint64 = 1
	iterSawConfig := false
	seenNodes := make(map[string]uint64)
	isFirstTry := true

	logDebugf("HTTP Looper starting.")
	for {
		routingInfo := c.routingInfo.get()
		if routingInfo == nil {
			// Shutdown the looper if the agent is shutdown
			break
		}

		var pickedSrv string
		for _, srv := range routingInfo.mgmtEpList {
			if seenNodes[srv] >= iterNum {
				continue
			}
			pickedSrv = srv
			break
		}

		if pickedSrv == "" {
			logDebugf("Pick Failed.")
			// All servers have been visited during this iteration
			if isFirstTry {
				logDebugf("Could not find any alive http hosts.")
				firstCfgFn(nil, ErrBadHosts)
				return
			} else {
				if !iterSawConfig {
					logDebugf("Looper waiting...")
					// Wait for a period before trying again if there was a problem...
					<-time.After(waitPeriod)
				}
				logDebugf("Looping again.")
				// Go to next iteration and try all servers again
				iterNum++
				iterSawConfig = false
				continue
			}
		}

		logDebugf("Http Picked: %s.", pickedSrv)

		seenNodes[pickedSrv] = iterNum

		hostname := hostnameFromUri(pickedSrv)

		logDebugf("HTTP Hostname: %s.", pickedSrv)

		var resp *http.Response
		// 1 on success, 0 on failure for node, -1 for generic failure
		var doConfigRequest func(bool) int

		doConfigRequest = func(is2x bool) int {
			streamPath := "bs"
			if is2x {
				streamPath = "bucketsStreaming"
			}
			// HTTP request time!
			uri := fmt.Sprintf("%s/pools/default/%s/%s", pickedSrv, streamPath, c.bucket)
			logDebugf("Requesting config from: %s.", uri)

			req, err := http.NewRequest("GET", uri, nil)
			if err != nil {
				logDebugf("Failed to build HTTP config request. %v", err)
				return 0
			}

			req.SetBasicAuth(c.bucket, c.password)

			resp, err = c.httpCli.Do(req)
			if err != nil {
				logDebugf("Failed to connect to host. %v", err)
				return 0
			}

			if resp.StatusCode != 200 {
				if resp.StatusCode == 401 {
					logDebugf("Failed to connect to host, bad auth.")
					firstCfgFn(nil, ErrAuthError)
					return -1
				} else if resp.StatusCode == 404 && !is2x {
					return doConfigRequest(true)
				}
				logDebugf("Failed to connect to host, unexpected status code: %v.", resp.StatusCode)
				return -1
			}
			return 1
		}

		switch doConfigRequest(false) {
		case 0:
			continue
		case -1:
			return
		}

		logDebugf("Connected.")

		// Autodisconnect eventually
		go func() {
			<-time.After(maxConnPeriod)
			logDebugf("Auto DC!")
			resp.Body.Close()
		}()

		dec := json.NewDecoder(resp.Body)
		configBlock := new(configStreamBlock)
		for {
			err := dec.Decode(configBlock)
			if err != nil {
				resp.Body.Close()
				break
			}

			logDebugf("Got Block: %v", string(configBlock.Bytes))

			bkCfg, err := parseConfig(configBlock.Bytes, hostname)
			if err != nil {
				logDebugf("Got error while parsing config: %v", err)
				resp.Body.Close()
				break
			}

			logDebugf("Got Config.")

			iterSawConfig = true
			if isFirstTry {
				logDebugf("HTTP Config Init")
				if !firstCfgFn(bkCfg, nil) {
					logDebugf("Got error while activating first config")
					resp.Body.Close()
					break
				}
				isFirstTry = false
			} else {
				logDebugf("HTTP Config Update")
				c.updateConfig(bkCfg)
			}
		}

		logDebugf("HTTP, Setting %s to iter %d", pickedSrv, iterNum)
	}
}
