package gocb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// View represents a Couchbase view within a design document.
type View struct {
	Map    string `json:"map,omitempty"`
	Reduce string `json:"reduce,omitempty"`
}

func (v View) hasReduce() bool {
	return v.Reduce != ""
}

// DesignDocument represents a Couchbase design document containing multiple views.
type DesignDocument struct {
	Name         string          `json:"-"`
	Views        map[string]View `json:"views,omitempty"`
	SpatialViews map[string]View `json:"spatial,omitempty"`
}

// IndexInfo represents a Couchbase GSI index.
type IndexInfo struct {
	Name      string    `json:"name"`
	IsPrimary bool      `json:"is_primary"`
	Type      IndexType `json:"using"`
	State     string    `json:"state"`
	Keyspace  string    `json:"keyspace_id"`
	Namespace string    `json:"namespace_id"`
	IndexKey  []string  `json:"index_key"`
}

// BucketManager provides methods for performing bucket management operations.
// See ClusterManager for methods that allow creating and removing buckets themselves.
type BucketManager struct {
	bucket   *Bucket
	username string
	password string
}

func (bm *BucketManager) capiRequest(method, uri, contentType string, body io.Reader) (*http.Response, error) {
	if contentType == "" && body != nil {
		panic("Content-type must be specified for non-null body.")
	}

	viewEp, err := bm.bucket.getViewEp()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, viewEp+uri, body)
	if contentType != "" {
		req.Header.Add("Content-Type", contentType)
	}
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(bm.username, bm.password)
	return bm.bucket.client.HttpClient().Do(req)
}

func (bm *BucketManager) mgmtRequest(method, uri, contentType string, body io.Reader) (*http.Response, error) {
	if contentType == "" && body != nil {
		panic("Content-type must be specified for non-null body.")
	}

	mgmtEp, err := bm.bucket.getMgmtEp()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, mgmtEp+uri, body)
	if contentType != "" {
		req.Header.Add("Content-Type", contentType)
	}
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(bm.username, bm.password)
	return bm.bucket.client.HttpClient().Do(req)
}

// Flush will delete all the of the data from a bucket.
// Keep in mind that you must have flushing enabled in the buckets configuration.
func (bm *BucketManager) Flush() error {
	reqUri := fmt.Sprintf("/pools/default/buckets/%s/controller/doFlush", bm.bucket.name)
	resp, err := bm.mgmtRequest("POST", reqUri, "", nil)
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

// GetDesignDocument retrieves a single design document for the given bucket..
func (bm *BucketManager) GetDesignDocument(name string) (*DesignDocument, error) {
	reqUri := fmt.Sprintf("/_design/%s", name)

	resp, err := bm.capiRequest("GET", reqUri, "", nil)
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

	ddocObj := DesignDocument{}
	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(&ddocObj)
	if err != nil {
		return nil, err
	}

	ddocObj.Name = name
	return &ddocObj, nil
}

// GetDesignDocuments will retrieve all design documents for the given bucket.
func (bm *BucketManager) GetDesignDocuments() ([]*DesignDocument, error) {
	reqUri := fmt.Sprintf("/pools/default/buckets/%s/ddocs", bm.bucket.name)

	resp, err := bm.mgmtRequest("GET", reqUri, "", nil)
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

	var ddocsObj struct {
		Rows []struct {
			Doc struct {
				Meta struct {
					Id string
				}
				Json DesignDocument
			}
		}
	}
	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(&ddocsObj)
	if err != nil {
		return nil, err
	}

	var ddocs []*DesignDocument
	for _, ddocData := range ddocsObj.Rows {
		ddoc := &ddocData.Doc.Json
		ddoc.Name = ddocData.Doc.Meta.Id[8:]
		ddocs = append(ddocs, ddoc)
	}

	return ddocs, nil
}

// InsertDesignDocument inserts a design document to the given bucket.
func (bm *BucketManager) InsertDesignDocument(ddoc *DesignDocument) error {
	oldDdoc, _ := bm.GetDesignDocument(ddoc.Name)
	if oldDdoc != nil {
		return clientError{"Design document already exists"}
	}
	return bm.UpsertDesignDocument(ddoc)
}

// UpsertDesignDocument will insert a design document to the given bucket, or update
// an existing design document with the same name.
func (bm *BucketManager) UpsertDesignDocument(ddoc *DesignDocument) error {
	reqUri := fmt.Sprintf("/_design/%s", ddoc.Name)

	data, err := json.Marshal(&ddoc)
	if err != nil {
		return err
	}

	resp, err := bm.capiRequest("PUT", reqUri, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return clientError{string(data)}
	}

	return nil
}

// RemoveDesignDocument will remove a design document from the given bucket.
func (bm *BucketManager) RemoveDesignDocument(name string) error {
	reqUri := fmt.Sprintf("/_design/%s", name)

	resp, err := bm.capiRequest("DELETE", reqUri, "", nil)
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

func (bm *BucketManager) createIndex(indexName string, fields []string, ignoreIfExists, deferred bool) error {
	var qs string

	if len(fields) == 0 {
		qs += "CREATE PRIMARY INDEX"
	} else {
		qs += "CREATE INDEX"
	}
	if indexName != "" {
		qs += " `" + indexName + "`"
	}
	qs += " ON `" + bm.bucket.name + "`"
	if len(fields) > 0 {
		qs += " ("
		for i := 0; i < len(fields); i++ {
			if i > 0 {
				qs += ", "
			}
			qs += "`" + fields[i] + "`"
		}
		qs += ")"
	}
	if deferred {
		qs += " WITH {\"defer_build\": true}"
	}

	rows, err := bm.bucket.ExecuteN1qlQuery(NewN1qlQuery(qs), nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exist") {
			if ignoreIfExists {
				return nil
			}
			return ErrIndexAlreadyExists
		}
		return err
	}

	if err := rows.Close(); err != nil {
		return err
	}
	return nil
}

// Creates an index over the specified fields.
func (bm *BucketManager) CreateIndex(indexName string, fields []string, ignoreIfExists, deferred bool) error {
	if indexName == "" {
		return ErrIndexInvalidName
	}
	if len(fields) <= 0 {
		return ErrIndexNoFields
	}
	return bm.createIndex(indexName, fields, ignoreIfExists, deferred)
}

// Creates a primary index.  An empty customName uses the default naming.
func (bm *BucketManager) CreatePrimaryIndex(customName string, ignoreIfExists, deferred bool) error {
	return bm.createIndex(customName, nil, ignoreIfExists, deferred)
}

func (bm *BucketManager) dropIndex(indexName string, ignoreIfNotExists bool) error {
	var qs string

	if indexName == "" {
		qs += "DROP PRIMARY INDEX `" + bm.bucket.name + "`"
	} else {
		qs += "DROP INDEX `" + bm.bucket.name + "`.`" + indexName + "`"
	}

	rows, err := bm.bucket.ExecuteN1qlQuery(NewN1qlQuery(qs), nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			if ignoreIfNotExists {
				return nil
			}
			return ErrIndexNotFound
		}
		return err
	}

	if err := rows.Close(); err != nil {
		return err
	}
	return nil
}

// Drops a specific index by name.
func (bm *BucketManager) DropIndex(indexName string, ignoreIfNotExists bool) error {
	if indexName == "" {
		return ErrIndexInvalidName
	}
	return bm.dropIndex(indexName, ignoreIfNotExists)
}

// Drops the primary index.  Pass an empty customName for unnamed primary indexes.
func (bm *BucketManager) DropPrimaryIndex(customName string, ignoreIfNotExists bool) error {
	return bm.dropIndex(customName, ignoreIfNotExists)
}

// Returns a list of all currently registered indexes.
func (bm *BucketManager) GetIndexes() ([]IndexInfo, error) {
	q := NewN1qlQuery("SELECT `indexes`.* FROM system:indexes")
	rows, err := bm.bucket.ExecuteN1qlQuery(q, nil)
	if err != nil {
		return nil, err
	}

	var indexes []IndexInfo
	var index IndexInfo
	for rows.Next(&index) {
		indexes = append(indexes, index)
		index = IndexInfo{}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	return indexes, nil
}

// Builds all indexes which are currently in deferred state.
func (bm *BucketManager) BuildDeferredIndexes() ([]string, error) {
	indexList, err := bm.GetIndexes()
	if err != nil {
		return nil, err
	}

	var deferredList []string
	for i := 0; i < len(indexList); i++ {
		var index = indexList[i]
		if index.State == "deferred" || index.State == "pending" {
			deferredList = append(deferredList, index.Name)
		}
	}

	if len(deferredList) == 0 {
		// Don't try to build an empty index list
		return nil, nil
	}

	var qs string
	qs += "BUILD INDEX ON `" + bm.bucket.name + "`("
	for i := 0; i < len(deferredList); i++ {
		if i > 0 {
			qs += ", "
		}
		qs += "`" + deferredList[i] + "`"
	}
	qs += ")"

	rows, err := bm.bucket.ExecuteN1qlQuery(NewN1qlQuery(qs), nil)
	if err != nil {
		return nil, err
	}

	if err := rows.Close(); err != nil {
		return nil, err
	}

	return deferredList, nil
}

func checkIndexesActive(indexes []IndexInfo, checkList []string) (bool, error) {
	var checkIndexes []IndexInfo
	for i := 0; i < len(checkList); i++ {
		indexName := checkList[i]

		for j := 0; j < len(indexes); j++ {
			if indexes[j].Name == indexName {
				checkIndexes = append(checkIndexes, indexes[j])
				break
			}
		}
	}

	if len(checkIndexes) != len(checkList) {
		return false, ErrIndexNotFound
	}

	for i := 0; i < len(checkIndexes); i++ {
		if checkIndexes[i].State != "online" {
			return false, nil
		}
	}
	return true, nil
}

// Waits for a set of indexes to come online
func (bm *BucketManager) WatchIndexes(watchList []string, watchPrimary bool, timeout time.Duration) error {
	if watchPrimary {
		watchList = append(watchList, "#primary")
	}

	curInterval := 50 * time.Millisecond
	timeoutTime := time.Now().Add(timeout)
	for {
		indexes, err := bm.GetIndexes()
		if err != nil {
			return err
		}

		allOnline, err := checkIndexesActive(indexes, watchList)
		if allOnline {
			break
		}

		curInterval += 500 * time.Millisecond
		if curInterval > 1000 {
			curInterval = 1000
		}

		if time.Now().Add(curInterval).After(timeoutTime) {
			return ErrTimeout
		}

		// Wait till our next poll interval
		time.Sleep(curInterval)
	}

	return nil
}
