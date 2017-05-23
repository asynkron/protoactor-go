package gocb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/brett19/jsonx.v1"
	"net/http"
	"time"
)

type n1qlCache struct {
	name        string
	encodedPlan string
}

type n1qlError struct {
	Code    uint32 `json:"code"`
	Message string `json:"msg"`
}

func (e *n1qlError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

type n1qlResponseMetrics struct {
	ElapsedTime   string `json:"elapsedTime"`
	ExecutionTime string `json:"executionTime"`
	ResultCount   uint   `json:"resultCount"`
	ResultSize    uint   `json:"resultSize"`
	MutationCount uint   `json:"mutationCount",omitempty`
	SortCount     uint   `json:"sortCount",omitempty`
	ErrorCount    uint   `json:"errorCount",omitempty`
	WarningCount  uint   `json:"warningCount",omitempty`
}

type n1qlResponse struct {
	RequestId       string              `json:"requestID"`
	ClientContextId string              `json:"clientContextID"`
	Results         []json.RawMessage   `json:"results,omitempty"`
	Errors          []n1qlError         `json:"errors,omitempty"`
	Status          string              `json:"status"`
	Metrics         n1qlResponseMetrics `json:"metrics"`
}

type n1qlMultiError []n1qlError

func (e *n1qlMultiError) Error() string {
	return (*e)[0].Error()
}

func (e *n1qlMultiError) Code() uint32 {
	return (*e)[0].Code
}

type QueryResultMetrics struct {
	ElapsedTime   time.Duration
	ExecutionTime time.Duration
	ResultCount   uint
	ResultSize    uint
	MutationCount uint
	SortCount     uint
	ErrorCount    uint
	WarningCount  uint
}

// QueryResults allows access to the results of a N1QL query.
type QueryResults interface {
	One(valuePtr interface{}) error
	Next(valuePtr interface{}) bool
	NextBytes() []byte
	Close() error

	RequestId() string
	ClientContextId() string
	Metrics() QueryResultMetrics
}

type n1qlResults struct {
	closed          bool
	index           int
	rows            []json.RawMessage
	err             error
	requestId       string
	clientContextId string
	metrics         QueryResultMetrics
}

func (r *n1qlResults) Next(valuePtr interface{}) bool {
	if r.err != nil {
		return false
	}

	row := r.NextBytes()
	if row == nil {
		return false
	}

	r.err = json.Unmarshal(row, valuePtr)
	if r.err != nil {
		return false
	}

	return true
}

func (r *n1qlResults) NextBytes() []byte {
	if r.err != nil {
		return nil
	}

	if r.index+1 >= len(r.rows) {
		r.closed = true
		return nil
	}
	r.index++

	return r.rows[r.index]
}

func (r *n1qlResults) Close() error {
	r.closed = true
	return r.err
}

func (r *n1qlResults) One(valuePtr interface{}) error {
	if !r.Next(valuePtr) {
		err := r.Close()
		if err != nil {
			return err
		}
		return ErrNoResults
	}
	// Ignore any errors occuring after we already have our result
	r.Close()
	// Return no error as we got the one result already.
	return nil
}

func (r *n1qlResults) RequestId() string {
	if !r.closed {
		panic("Result must be closed before accessing meta-data")
	}

	return r.requestId
}

func (r *n1qlResults) ClientContextId() string {
	if !r.closed {
		panic("Result must be closed before accessing meta-data")
	}

	return r.clientContextId
}

func (r *n1qlResults) Metrics() QueryResultMetrics {
	if !r.closed {
		panic("Result must be closed before accessing meta-data")
	}

	return r.metrics
}

// Executes the N1QL query (in opts) on the server n1qlEp.
// This function assumes that `opts` already contains all the required
// settings. This function will inject any additional connection or request-level
// settings into the `opts` map (currently this is only the timeout).
func (c *Cluster) executeN1qlQuery(n1qlEp string, opts map[string]interface{}, creds []userPassPair, timeout time.Duration, client *http.Client) (QueryResults, error) {
	reqUri := fmt.Sprintf("%s/query/service", n1qlEp)

	tmostr, castok := opts["timeout"].(string)
	if castok {
		var err error
		timeout, err = time.ParseDuration(tmostr)
		if err != nil {
			return nil, err
		}
	} else {
		// Set the timeout string to its default variant
		opts["timeout"] = timeout.String()
	}

	if len(creds) > 1 {
		opts["creds"] = creds
	}

	reqJson, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", reqUri, bytes.NewBuffer(reqJson))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if len(creds) == 1 {
		req.SetBasicAuth(creds[0].Username, creds[0].Password)
	}

	resp, err := doHttpWithTimeout(client, req, timeout)
	if err != nil {
		return nil, err
	}

	n1qlResp := n1qlResponse{}
	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(&n1qlResp)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	if len(n1qlResp.Errors) > 0 {
		return nil, (*n1qlMultiError)(&n1qlResp.Errors)
	}

	if resp.StatusCode != 200 {
		return nil, &viewError{
			Message: "HTTP Error",
			Reason:  fmt.Sprintf("Status code was %d.", resp.StatusCode),
		}
	}

	elapsedTime, _ := time.ParseDuration(n1qlResp.Metrics.ElapsedTime)
	executionTime, _ := time.ParseDuration(n1qlResp.Metrics.ExecutionTime)

	return &n1qlResults{
		requestId:       n1qlResp.RequestId,
		clientContextId: n1qlResp.ClientContextId,
		index:           -1,
		rows:            n1qlResp.Results,
		metrics: QueryResultMetrics{
			ElapsedTime:   elapsedTime,
			ExecutionTime: executionTime,
			ResultCount:   n1qlResp.Metrics.ResultCount,
			ResultSize:    n1qlResp.Metrics.ResultSize,
			MutationCount: n1qlResp.Metrics.MutationCount,
			SortCount:     n1qlResp.Metrics.SortCount,
			ErrorCount:    n1qlResp.Metrics.ErrorCount,
			WarningCount:  n1qlResp.Metrics.WarningCount,
		},
	}, nil
}

func (c *Cluster) prepareN1qlQuery(n1qlEp string, opts map[string]interface{}, creds []userPassPair, timeout time.Duration, client *http.Client) (*n1qlCache, error) {
	prepOpts := make(map[string]interface{})
	for k, v := range opts {
		prepOpts[k] = v
	}
	prepOpts["statement"] = "PREPARE " + opts["statement"].(string)

	prepRes, err := c.executeN1qlQuery(n1qlEp, prepOpts, creds, timeout, client)
	if err != nil {
		return nil, err
	}

	var preped n1qlPrepData
	err = prepRes.One(&preped)
	if err != nil {
		return nil, err
	}

	return &n1qlCache{
		name:        preped.Name,
		encodedPlan: preped.EncodedPlan,
	}, nil
}

type n1qlPrepData struct {
	EncodedPlan string `json:"encoded_plan"`
	Name        string `json:"name"`
}

// Performs a spatial query and returns a list of rows or an error.
func (c *Cluster) doN1qlQuery(b *Bucket, q *N1qlQuery, params interface{}) (QueryResults, error) {
	var err error
	var n1qlEp string
	var timeout time.Duration
	var client *http.Client
	var creds []userPassPair

	if b != nil {
		n1qlEp, err = b.getN1qlEp()
		if err != nil {
			return nil, err
		}

		if b.n1qlTimeout < c.n1qlTimeout {
			timeout = b.n1qlTimeout
		} else {
			timeout = c.n1qlTimeout
		}
		client = b.client.HttpClient()
		if c.auth != nil {
			creds = c.auth.bucketN1ql(b.name)
		} else {
			creds = []userPassPair{
				userPassPair{
					Username: b.name,
					Password: b.password,
				},
			}
		}
	} else {
		if c.auth == nil {
			panic("Cannot perform cluster level queries without Cluster Authenticator.")
		}

		tmpB, err := c.randomBucket()
		if err != nil {
			return nil, err
		}

		n1qlEp, err = tmpB.getN1qlEp()
		if err != nil {
			return nil, err
		}

		timeout = c.n1qlTimeout
		client = tmpB.client.HttpClient()
		creds = c.auth.clusterN1ql()
	}

	execOpts := make(map[string]interface{})
	for k, v := range q.options {
		execOpts[k] = v
	}
	if params != nil {
		args, isArray := params.([]interface{})
		if isArray {
			execOpts["args"] = args
		} else {
			mapArgs, isMap := params.(map[string]interface{})
			if isMap {
				for key, value := range mapArgs {
					execOpts["$"+key] = value
				}
			} else {
				panic("Invalid params argument passed")
			}
		}
	}

	if q.adHoc {
		return c.executeN1qlQuery(n1qlEp, execOpts, creds, timeout, client)
	}

	// Do Prepared Statement Logic
	var cachedStmt *n1qlCache

	stmtStr := q.options["statement"].(string)

	c.clusterLock.RLock()
	cachedStmt = c.queryCache[stmtStr]
	c.clusterLock.RUnlock()

	if cachedStmt != nil {
		// Attempt to execute our cached query plan
		delete(execOpts, "statement")
		execOpts["prepared"] = cachedStmt.name
		execOpts["encoded_plan"] = cachedStmt.encodedPlan

		results, err := c.executeN1qlQuery(n1qlEp, execOpts, creds, timeout, client)
		if err == nil {
			return results, nil
		}

		// If we get error 4050, 4070 or 5000, we should attempt
		//   to reprepare the statement immediately before failing.
		n1qlErr, isN1qlErr := err.(*n1qlMultiError)
		if !isN1qlErr {
			return nil, err
		}
		if n1qlErr.Code() != 4050 && n1qlErr.Code() != 4070 && n1qlErr.Code() != 5000 {
			return nil, err
		}
	}

	// Prepare the query
	cachedStmt, err = c.prepareN1qlQuery(n1qlEp, q.options, creds, timeout, client)
	if err != nil {
		return nil, err
	}

	// Save new cached statement
	c.clusterLock.Lock()
	c.queryCache[stmtStr] = cachedStmt
	c.clusterLock.Unlock()

	// Update with new prepared data
	delete(execOpts, "statement")
	execOpts["prepared"] = cachedStmt.name
	execOpts["encoded_plan"] = cachedStmt.encodedPlan

	return c.executeN1qlQuery(n1qlEp, execOpts, creds, timeout, client)
}

// Performs a n1ql query and returns a list of rows or an error.
func (c *Cluster) ExecuteN1qlQuery(q *N1qlQuery, params interface{}) (QueryResults, error) {
	return c.doN1qlQuery(nil, q, params)
}

// SearchResultLocation holds the location of a hit in a list of search results.
type SearchResultLocation struct {
	Position       int    `json:"position,omitempty"`
	Start          int    `json:"start,omitempty"`
	End            int    `json:"end,omitempty"`
	ArrayPositions []uint `json:"array_positions,omitempty"`
}

// SearchResultHit holds a single hit in a list of search results.
type SearchResultHit struct {
	Index       string                                       `json:"index,omitempty"`
	Id          string                                       `json:"id,omitempty"`
	Score       float64                                      `json:"score,omitempty"`
	Explanation map[string]interface{}                       `json:"explanation,omitempty"`
	Locations   map[string]map[string][]SearchResultLocation `json:"locations,omitempty"`
	Fragments   map[string][]string                          `json:"fragments,omitempty"`
	Fields      map[string]string                            `json:"fields,omitempty"`
}

// SearchResultTermFacet holds the results of a term facet in search results.
type SearchResultTermFacet struct {
	Term  string `json:"term,omitempty"`
	Count int    `json:"count,omitempty"`
}

// SearchResultNumericFacet holds the results of a numeric facet in search results.
type SearchResultNumericFacet struct {
	Name  string  `json:"name,omitempty"`
	Min   float64 `json:"min,omitempty"`
	Max   float64 `json:"max,omitempty"`
	Count int     `json:"count,omitempty"`
}

// SearchResultDateFacet holds the results of a date facet in search results.
type SearchResultDateFacet struct {
	Name  string `json:"name,omitempty"`
	Min   string `json:"min,omitempty"`
	Max   string `json:"max,omitempty"`
	Count int    `json:"count,omitempty"`
}

// SearchResultFacet holds the results of a specified facet in search results.
type SearchResultFacet struct {
	Field         string                     `json:"field,omitempty"`
	Total         int                        `json:"total,omitempty"`
	Missing       int                        `json:"missing,omitempty"`
	Other         int                        `json:"missing,omitempty"`
	Terms         []SearchResultTermFacet    `json:"terms,omitempty"`
	NumericRanges []SearchResultNumericFacet `json:"numeric_ranges,omitempty"`
	DateRanges    []SearchResultDateFacet    `json:"date_ranges,omitempty"`
}

// SearchResultStatus holds the status information for an executed search query.
type SearchResultStatus struct {
	Total      int `json:"total,omitempty"`
	Failed     int `json:"failed,omitempty"`
	Successful int `json:"successful,omitempty"`
}

// *VOLATILE*
// SearchResults allows access to the results of a search query.
type SearchResults interface {
	Status() SearchResultStatus
	Errors() []string
	TotalHits() int
	Hits() []SearchResultHit
	Facets() map[string]SearchResultFacet
	Took() time.Duration
	MaxScore() float64
}

type searchResponse struct {
	Status    SearchResultStatus           `json:"status,omitempty"`
	Errors    []string                     `json:"errors,omitempty"`
	TotalHits int                          `json:"total_hits,omitempty"`
	Hits      []SearchResultHit            `json:"hits,omitempty"`
	Facets    map[string]SearchResultFacet `json:"facets,omitempty"`
	Took      uint                         `json:"took,omitempty"`
	MaxScore  float64                      `json:"max_score,omitempty"`
}

type searchResults struct {
	data *searchResponse
}

func (r searchResults) Status() SearchResultStatus {
	return r.data.Status
}
func (r searchResults) Errors() []string {
	return r.data.Errors
}
func (r searchResults) TotalHits() int {
	return r.data.TotalHits
}
func (r searchResults) Hits() []SearchResultHit {
	return r.data.Hits
}
func (r searchResults) Facets() map[string]SearchResultFacet {
	return r.data.Facets
}
func (r searchResults) Took() time.Duration {
	return time.Duration(r.data.Took) / time.Nanosecond
}
func (r searchResults) MaxScore() float64 {
	return r.data.MaxScore
}

// Performs a spatial query and returns a list of rows or an error.
func (c *Cluster) doSearchQuery(b *Bucket, q *SearchQuery) (SearchResults, error) {
	var err error
	var ftsEp string
	var timeout time.Duration
	var client *http.Client
	var creds []userPassPair

	if b != nil {
		ftsEp, err = b.getFtsEp()
		if err != nil {
			return nil, err
		}

		if b.ftsTimeout < c.ftsTimeout {
			timeout = b.ftsTimeout
		} else {
			timeout = c.ftsTimeout
		}
		client = b.client.HttpClient()
		if c.auth != nil {
			creds = c.auth.bucketFts(b.name)
		} else {
			creds = []userPassPair{
				userPassPair{
					Username: b.name,
					Password: b.password,
				},
			}
		}
	} else {
		if c.auth == nil {
			panic("Cannot perform cluster level queries without Cluster Authenticator.")
		}

		tmpB, err := c.randomBucket()
		if err != nil {
			return nil, err
		}

		ftsEp, err = tmpB.getFtsEp()
		if err != nil {
			return nil, err
		}

		timeout = c.ftsTimeout
		client = tmpB.client.HttpClient()
		creds = c.auth.clusterFts()
	}

	qIndexName := q.indexName()
	qBytes, err := json.Marshal(q.queryData())
	if err != nil {
		return nil, err
	}

	var queryData jsonx.DelayedObject
	err = json.Unmarshal(qBytes, &queryData)
	if err != nil {
		return nil, err
	}

	var ctlData jsonx.DelayedObject
	if queryData.Has("ctl") {
		err = queryData.Get("ctl", &ctlData)
		if err != nil {
			return nil, err
		}
	}

	qTimeout := jsonMillisecondDuration(timeout)
	if ctlData.Has("timeout") {
		err := ctlData.Get("timeout", &qTimeout)
		if err != nil {
			return nil, err
		}
		if qTimeout <= 0 || time.Duration(qTimeout) > timeout {
			qTimeout = jsonMillisecondDuration(timeout)
		}
	}
	ctlData.Set("timeout", qTimeout)

	queryData.Set("ctl", ctlData)

	if len(creds) > 1 {
		queryData.Set("creds", creds)
	}

	qBytes, err = json.Marshal(queryData)
	if err != nil {
		return nil, err
	}

	reqUri := fmt.Sprintf("%s/api/index/%s/query", ftsEp, qIndexName)

	req, err := http.NewRequest("POST", reqUri, bytes.NewBuffer(qBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if len(creds) == 1 {
		req.SetBasicAuth(creds[0].Username, creds[0].Password)
	}

	resp, err := doHttpWithTimeout(client, req, timeout)
	if err != nil {
		return nil, err
	}

	ftsResp := searchResponse{}
	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(&ftsResp)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, &viewError{
			Message: "HTTP Error",
			Reason:  fmt.Sprintf("Status code was %d.", resp.StatusCode),
		}
	}

	return searchResults{
		data: &ftsResp,
	}, nil
}

// *VOLATILE*
// Performs a n1ql query and returns a list of rows or an error.
func (c *Cluster) ExecuteSearchQuery(q *SearchQuery) (SearchResults, error) {
	return c.doSearchQuery(nil, q)
}
