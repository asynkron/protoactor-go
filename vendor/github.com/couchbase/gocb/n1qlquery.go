package gocb

import (
	"time"
)

// ConsistencyMode indicates the level of data consistency desired for a query.
type ConsistencyMode int

const (
	// NotBounded indicates no data consistency is required.
	NotBounded = ConsistencyMode(1)
	// RequestPlus indicates that request-level data consistency is required.
	RequestPlus = ConsistencyMode(2)
	// StatementPlus inidcates that statement-level data consistency is required.
	StatementPlus = ConsistencyMode(3)
)

// N1qlQuery represents a pending N1QL query.
type N1qlQuery struct {
	options map[string]interface{}
	adHoc   bool
}

// Consistency specifies the level of consistency required for this query.
func (nq *N1qlQuery) Consistency(stale ConsistencyMode) *N1qlQuery {
	if _, ok := nq.options["scan_vectors"]; ok {
		panic("Consistent and ConsistentWith must be used exclusively")
	}
	if stale == NotBounded {
		nq.options["scan_consistency"] = "not_bounded"
	} else if stale == RequestPlus {
		nq.options["scan_consistency"] = "request_plus"
	} else if stale == StatementPlus {
		nq.options["scan_consistency"] = "statement_plus"
	} else {
		panic("Unexpected consistency option")
	}
	return nq
}

// ConsistentWith specifies a mutation state to be consistent with for this query.
func (nq *N1qlQuery) ConsistentWith(state *MutationState) *N1qlQuery {
	if _, ok := nq.options["scan_consistency"]; ok {
		panic("Consistent and ConsistentWith must be used exclusively")
	}
	nq.options["scan_consistency"] = "at_plus"
	nq.options["scan_vectors"] = state
	return nq
}

// AdHoc specifies that this query is adhoc and should not be prepared.
func (nq *N1qlQuery) AdHoc(adhoc bool) *N1qlQuery {
	nq.adHoc = adhoc
	return nq
}

// Custom allows specifying custom query options.
func (nq *N1qlQuery) Custom(name string, value interface{}) *N1qlQuery {
	nq.options[name] = value
	return nq
}

// Timeout indicates the maximum time to wait for this query to complete.
func (nq *N1qlQuery) Timeout(timeout time.Duration) *N1qlQuery {
	nq.options["timeout"] = timeout.String()
	return nq
}

// NewN1qlQuery creates a new N1qlQuery object from a query string.
func NewN1qlQuery(statement string) *N1qlQuery {
	nq := &N1qlQuery{
		options: make(map[string]interface{}),
		adHoc:   true,
	}
	nq.options["statement"] = statement
	return nq
}
