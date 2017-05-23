package gocb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
)

// ClusterManager provides methods for performing cluster management operations.
type ClusterManager struct {
	hosts    []string
	username string
	password string
	httpCli  *http.Client
}

// BucketType specifies the kind of bucket
type BucketType int

const (
	// Couchbase indicates a Couchbase bucket type.
	Couchbase = BucketType(0)

	// Memcached indicates a Memcached bucket type.
	Memcached = BucketType(1)
)

type bucketDataIn struct {
	Name         string `json:"name"`
	BucketType   string `json:"bucketType"`
	AuthType     string `json:"authType"`
	SaslPassword string `json:"saslPassword"`
	Quota        struct {
		Ram    int `json:"ram"`
		RawRam int `json:"rawRAM"`
	} `json:"quota"`
	ReplicaNumber int  `json:"replicaNumber"`
	ReplicaIndex  bool `json:"replicaIndex"`
	Controllers   struct {
		Flush string `json:"flush"`
	} `json:"controllers"`
}

// BucketSettings holds information about the settings for a bucket.
type BucketSettings struct {
	FlushEnabled  bool
	IndexReplicas bool
	Name          string
	Password      string
	Quota         int
	Replicas      int
	Type          BucketType
}

func (cm *ClusterManager) getMgmtEp() string {
	return cm.hosts[rand.Intn(len(cm.hosts))]
}

func (cm *ClusterManager) mgmtRequest(method, uri string, contentType string, body io.Reader) (*http.Response, error) {
	if contentType == "" && body != nil {
		panic("Content-type must be specified for non-null body.")
	}

	reqUri := cm.getMgmtEp() + uri
	req, err := http.NewRequest(method, reqUri, body)
	if contentType != "" {
		req.Header.Add("Content-Type", contentType)
	}
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cm.username, cm.password)
	return cm.httpCli.Do(req)
}

func bucketDataInToSettings(bucketData *bucketDataIn) *BucketSettings {
	settings := &BucketSettings{
		FlushEnabled:  bucketData.Controllers.Flush != "",
		IndexReplicas: bucketData.ReplicaIndex,
		Name:          bucketData.Name,
		Password:      bucketData.SaslPassword,
		Quota:         bucketData.Quota.Ram,
		Replicas:      bucketData.ReplicaNumber,
	}
	if bucketData.BucketType == "membase" {
		settings.Type = Couchbase
	} else if bucketData.BucketType == "memcached" {
		settings.Type = Memcached
	} else {
		panic("Unrecognized bucket type string.")
	}
	if bucketData.AuthType != "sasl" {
		settings.Password = ""
	}
	return settings
}

// GetBuckets returns a list of all active buckets on the cluster.
func (cm *ClusterManager) GetBuckets() ([]*BucketSettings, error) {
	resp, err := cm.mgmtRequest("GET", "/pools/default/buckets", "", nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
		return nil, clientError{string(data)}
	}

	var bucketsData []*bucketDataIn
	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(&bucketsData)
	if err != nil {
		return nil, err
	}

	var buckets []*BucketSettings
	for _, bucketData := range bucketsData {
		buckets = append(buckets, bucketDataInToSettings(bucketData))
	}

	return buckets, nil
}

// InsertBucket creates a new bucket on the cluster.
func (cm *ClusterManager) InsertBucket(settings *BucketSettings) error {
	posts := url.Values{}
	posts.Add("name", settings.Name)
	if settings.Type == Couchbase {
		posts.Add("bucketType", "couchbase")
	} else if settings.Type == Memcached {
		posts.Add("bucketType", "memcached")
	} else {
		panic("Unrecognized bucket type.")
	}
	if settings.FlushEnabled {
		posts.Add("flushEnabled", "1")
	} else {
		posts.Add("flushEnabled", "0")
	}
	posts.Add("replicaNumber", fmt.Sprintf("%d", settings.Replicas))
	posts.Add("authType", "sasl")
	posts.Add("saslPassword", settings.Password)
	posts.Add("ramQuotaMB", fmt.Sprintf("%d", settings.Quota))
	posts.Add("proxyPort", "11210")

	data := []byte(posts.Encode())
	resp, err := cm.mgmtRequest("POST", "/pools/default/buckets", "application/x-www-form-urlencoded", bytes.NewReader(data))
	if err != nil {
		return nil
	}

	if resp.StatusCode != 202 {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return clientError{string(data)}
	}

	return nil
}

// UpdateBucket will update the settings for a specific bucket on the cluster.
func (cm *ClusterManager) UpdateBucket(settings *BucketSettings) error {
	// Cluster-side, updates are the same as creates.
	return cm.InsertBucket(settings)
}

// RemoveBucket will delete a bucket from the cluster by name.
func (cm *ClusterManager) RemoveBucket(name string) error {
	reqUri := fmt.Sprintf("/pools/default/buckets/%s", name)

	resp, err := cm.mgmtRequest("DELETE", reqUri, "", nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return clientError{string(data)}
	}

	return nil
}
