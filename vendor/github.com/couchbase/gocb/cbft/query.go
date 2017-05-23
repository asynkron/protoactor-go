package cbft

import (
	"encoding/json"
)

// *VOLATILE*
// FtsQuery represents an FTS query for a search query.
type FtsQuery interface {
}

type ftsQueryBase struct {
	options map[string]interface{}
}

func newFtsQueryBase() ftsQueryBase {
	return ftsQueryBase{
		options: make(map[string]interface{}),
	}
}

// MarshalJSON marshal's this query to JSON for the FTS REST API.
func (q ftsQueryBase) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.options)
}

// MatchQuery represents a FTS match query.
type MatchQuery struct {
	ftsQueryBase
}

// NewMatchQuery creates a new MatchQuery.
func NewMatchQuery(match string) *MatchQuery {
	q := &MatchQuery{newFtsQueryBase()}
	q.options["match"] = match
	return q
}

// Field specifies the field for this query.
func (q *MatchQuery) Field(field string) *MatchQuery {
	q.options["field"] = field
	return q
}

// Analyzer specifies the analyzer to use for this query.
func (q *MatchQuery) Analyzer(analyzer string) *MatchQuery {
	q.options["analyzer"] = analyzer
	return q
}

// PrefixLength specifies the prefix length from this query.
func (q *MatchQuery) PrefixLength(length int) *MatchQuery {
	q.options["prefix_length"] = length
	return q
}

// Fuzziness specifies the fuziness for this query.
func (q *MatchQuery) Fuzziness(fuzziness int) *MatchQuery {
	q.options["fuzziness"] = fuzziness
	return q
}

// Boost specifies the boost for this query.
func (q *MatchQuery) Boost(boost float32) *MatchQuery {
	q.options["boost"] = boost
	return q
}

// MatchPhraseQuery represents a FTS match phrase query.
type MatchPhraseQuery struct {
	ftsQueryBase
}

// NewMatchPhraseQuery creates a new MatchPhraseQuery
func NewMatchPhraseQuery(phrase string) *MatchPhraseQuery {
	q := &MatchPhraseQuery{newFtsQueryBase()}
	q.options["match_phrase"] = phrase
	return q
}

// Field specifies the field for this query.
func (q *MatchPhraseQuery) Field(field string) *MatchPhraseQuery {
	q.options["field"] = field
	return q
}

// Analyzer specifies the analyzer to use for this query.
func (q *MatchPhraseQuery) Analyzer(analyzer string) *MatchPhraseQuery {
	q.options["analyzer"] = analyzer
	return q
}

// Boost specifies the boost for this query.
func (q *MatchPhraseQuery) Boost(boost float32) *MatchPhraseQuery {
	q.options["boost"] = boost
	return q
}

// RegexpQuery represents a FTS regular expression query.
type RegexpQuery struct {
	ftsQueryBase
}

// NewRegexpQuery creates a new RegexpQuery.
func NewRegexpQuery(regexp string) *RegexpQuery {
	q := &RegexpQuery{newFtsQueryBase()}
	q.options["regexp"] = regexp
	return q
}

// Field specifies the field for this query.
func (q *RegexpQuery) Field(field string) *RegexpQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *RegexpQuery) Boost(boost float32) *RegexpQuery {
	q.options["boost"] = boost
	return q
}

// StringQuery represents a FTS string query.
type QueryStringQuery struct {
	ftsQueryBase
}

// NewStringQuery creates a new StringQuery.
func NewQueryStringQuery(query string) *QueryStringQuery {
	q := &QueryStringQuery{newFtsQueryBase()}
	q.options["query"] = query
	return q
}

// Boost specifies the boost for this query.
func (q *QueryStringQuery) Boost(boost float32) *QueryStringQuery {
	q.options["boost"] = boost
	return q
}

// NumericRangeQuery represents a FTS numeric range query.
type NumericRangeQuery struct {
	ftsQueryBase
}

// NewNumericRangeQuery creates a new NumericRangeQuery.
func NewNumericRangeQuery() *NumericRangeQuery {
	q := &NumericRangeQuery{newFtsQueryBase()}
	return q
}

// Min specifies the minimum value and inclusiveness for this range query.
func (q *NumericRangeQuery) Min(min float32, inclusive bool) *NumericRangeQuery {
	q.options["min"] = min
	q.options["inclusive_min"] = inclusive
	return q
}

// Max specifies the maximum value and inclusiveness for this range query.
func (q *NumericRangeQuery) Max(max float32, inclusive bool) *NumericRangeQuery {
	q.options["max"] = max
	q.options["inclusive_max"] = inclusive
	return q
}

// Field specifies the field for this query.
func (q *NumericRangeQuery) Field(field string) *NumericRangeQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *NumericRangeQuery) Boost(boost float32) *NumericRangeQuery {
	q.options["boost"] = boost
	return q
}

// DateRangeQuery represents a FTS date range query.
type DateRangeQuery struct {
	ftsQueryBase
}

// NewDateRangeQuery creates a new DateRangeQuery.
func NewDateRangeQuery() *DateRangeQuery {
	q := &DateRangeQuery{newFtsQueryBase()}
	return q
}

// Start specifies the start value and inclusiveness for this range query.
func (q *DateRangeQuery) Start(start string, inclusive bool) *DateRangeQuery {
	q.options["start"] = start
	q.options["inclusive_start"] = inclusive
	return q
}

// End specifies the end value and inclusiveness for this range query.
func (q *DateRangeQuery) End(end string, inclusive bool) *DateRangeQuery {
	q.options["end"] = end
	q.options["inclusive_end"] = inclusive
	return q
}

// DateTimeParser specifies which date time string parser to use.
func (q *DateRangeQuery) DateTimeParser(parser string) *DateRangeQuery {
	q.options["datetime_parser"] = parser
	return q
}

// Field specifies the field for this query.
func (q *DateRangeQuery) Field(field string) *DateRangeQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *DateRangeQuery) Boost(boost float32) *DateRangeQuery {
	q.options["boost"] = boost
	return q
}

// ConjunctionQuery represents a FTS conjunction query.
type ConjunctionQuery struct {
	ftsQueryBase
}

// NewConjunctionQuery creates a new ConjunctionQuery.
func NewConjunctionQuery(queries ...FtsQuery) *ConjunctionQuery {
	q := &ConjunctionQuery{newFtsQueryBase()}
	q.options["conjuncts"] = []FtsQuery{}
	return q.And(queries...)
}

// And adds new predicate queries to this conjunction query.
func (q *ConjunctionQuery) And(queries ...FtsQuery) *ConjunctionQuery {
	q.options["conjuncts"] = append(q.options["conjuncts"].([]FtsQuery), queries...)
	return q
}

// Boost specifies the boost for this query.
func (q *ConjunctionQuery) Boost(boost float32) *ConjunctionQuery {
	q.options["boost"] = boost
	return q
}

// DisjunctionQuery represents a FTS disjunction query.
type DisjunctionQuery struct {
	ftsQueryBase
}

// NewDisjunctionQuery creates a new DisjunctionQuery.
func NewDisjunctionQuery(queries ...FtsQuery) *DisjunctionQuery {
	q := &DisjunctionQuery{newFtsQueryBase()}
	q.options["disjuncts"] = []FtsQuery{}
	return q.Or(queries...)
}

// Or adds new predicate queries to this disjunction query.
func (q *DisjunctionQuery) Or(queries ...FtsQuery) *DisjunctionQuery {
	q.options["disjuncts"] = append(q.options["disjuncts"].([]FtsQuery), queries...)
	return q
}

// Boost specifies the boost for this query.
func (q *DisjunctionQuery) Boost(boost float32) *DisjunctionQuery {
	q.options["boost"] = boost
	return q
}

type booleanQueryData struct {
	Must    *ConjunctionQuery `json:"must,omitempty"`
	Should  *DisjunctionQuery `json:"should,omitempty"`
	MustNot *DisjunctionQuery `json:"must_not,omitempty"`
	Boost   float32           `json:"boost,omitempty"`
}

// BooleanQuery represents a FTS boolean query.
type BooleanQuery struct {
	data      booleanQueryData
	shouldMin int
}

// NewBooleanQuery creates a new BooleanQuery.
func NewBooleanQuery() *BooleanQuery {
	q := &BooleanQuery{}
	return q
}

// Must specifies a query which must match.
func (q *BooleanQuery) Must(query FtsQuery) *BooleanQuery {
	switch val := query.(type) {
	case ConjunctionQuery:
		query = &val
	case *ConjunctionQuery:
		// Do nothing
	default:
		query = NewConjunctionQuery(val)
	}
	q.data.Must = query.(*ConjunctionQuery)
	return q
}

// Should specifies a query which should match.
func (q *BooleanQuery) Should(query FtsQuery) *BooleanQuery {
	switch val := query.(type) {
	case DisjunctionQuery:
		query = &val
	case *DisjunctionQuery:
	// Do nothing
	default:
		query = NewDisjunctionQuery(val)
	}
	q.data.Should = query.(*DisjunctionQuery)
	return q
}

// MustNot specifies a query which must not match.
func (q *BooleanQuery) MustNot(query FtsQuery) *BooleanQuery {
	switch val := query.(type) {
	case DisjunctionQuery:
		query = &val
	case *DisjunctionQuery:
	// Do nothing
	default:
		query = NewDisjunctionQuery(val)
	}
	q.data.MustNot = query.(*DisjunctionQuery)
	return q
}

// ShouldMin specifies the minimum value before the should query will boost.
func (q *BooleanQuery) ShouldMin(min int) *BooleanQuery {
	q.shouldMin = min
	return q
}

// Boost specifies the boost for this query.
func (q *BooleanQuery) Boost(boost float32) *BooleanQuery {
	q.data.Boost = boost
	return q
}

// MarshalJSON marshal's this query to JSON for the FTS REST API.
func (q *BooleanQuery) MarshalJSON() ([]byte, error) {
	if q.data.Should != nil {
		q.data.Should.options["min"] = q.shouldMin
	}
	bytes, err := json.Marshal(q.data)
	if q.data.Should != nil {
		delete(q.data.Should.options, "min")
	}
	return bytes, err
}

// WildcardQuery represents a FTS wildcard query.
type WildcardQuery struct {
	ftsQueryBase
}

// NewWildcardQuery creates a new WildcardQuery.
func NewWildcardQuery(wildcard string) *WildcardQuery {
	q := &WildcardQuery{newFtsQueryBase()}
	q.options["wildcard"] = wildcard
	return q
}

// Field specifies the field for this query.
func (q *WildcardQuery) Field(field string) *WildcardQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *WildcardQuery) Boost(boost float32) *WildcardQuery {
	q.options["boost"] = boost
	return q
}

// DocIdQuery represents a FTS document id query.
type DocIdQuery struct {
	ftsQueryBase
}

// NewDocIdQuery creates a new DocIdQuery.
func NewDocIdQuery(ids ...string) *DocIdQuery {
	q := &DocIdQuery{newFtsQueryBase()}
	q.options["ids"] = []string{}
	return q.AddDocIds(ids...)
}

// AddDocIds adds addition document ids to this query.
func (q *DocIdQuery) AddDocIds(ids ...string) *DocIdQuery {
	q.options["ids"] = append(q.options["ids"].([]string), ids...)
	return q
}

// Field specifies the field for this query.
func (q *DocIdQuery) Field(field string) *DocIdQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *DocIdQuery) Boost(boost float32) *DocIdQuery {
	q.options["boost"] = boost
	return q
}

// BooleanFieldQuery represents a FTS boolean field query.
type BooleanFieldQuery struct {
	ftsQueryBase
}

// NewBooleanFieldQuery creates a new BooleanFieldQuery.
func NewBooleanFieldQuery(val bool) *BooleanFieldQuery {
	q := &BooleanFieldQuery{newFtsQueryBase()}
	q.options["bool"] = val
	return q
}

// Field specifies the field for this query.
func (q *BooleanFieldQuery) Field(field string) *BooleanFieldQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *BooleanFieldQuery) Boost(boost float32) *BooleanFieldQuery {
	q.options["boost"] = boost
	return q
}

// TermQuery represents a FTS term query.
type TermQuery struct {
	ftsQueryBase
}

// NewTermQuery creates a new TermQuery.
func NewTermQuery(term string) *TermQuery {
	q := &TermQuery{newFtsQueryBase()}
	q.options["term"] = term
	return q
}

// Field specifies the field for this query.
func (q *TermQuery) Field(field string) *TermQuery {
	q.options["field"] = field
	return q
}

// PrefixLength specifies the prefix length from this query.
func (q *TermQuery) PrefixLength(length int) *TermQuery {
	q.options["prefix_length"] = length
	return q
}

// Fuzziness specifies the fuziness for this query.
func (q *TermQuery) Fuzziness(fuzziness int) *TermQuery {
	q.options["fuzziness"] = fuzziness
	return q
}

// Boost specifies the boost for this query.
func (q *TermQuery) Boost(boost float32) *TermQuery {
	q.options["boost"] = boost
	return q
}

// PhraseQuery represents a FTS phrase query.
type PhraseQuery struct {
	ftsQueryBase
}

// NewPhraseQuery creates a new PhraseQuery.
func NewPhraseQuery(terms ...string) *PhraseQuery {
	q := &PhraseQuery{newFtsQueryBase()}
	q.options["terms"] = terms
	return q
}

// Field specifies the field for this query.
func (q *PhraseQuery) Field(field string) *PhraseQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *PhraseQuery) Boost(boost float32) *PhraseQuery {
	q.options["boost"] = boost
	return q
}

// PrefixQuery represents a FTS prefix query.
type PrefixQuery struct {
	ftsQueryBase
}

// NewPrefixQuery creates a new PrefixQuery.
func NewPrefixQuery(prefix string) *PrefixQuery {
	q := &PrefixQuery{newFtsQueryBase()}
	q.options["prefix"] = prefix
	return q
}

// Field specifies the field for this query.
func (q *PrefixQuery) Field(field string) *PrefixQuery {
	q.options["field"] = field
	return q
}

// Boost specifies the boost for this query.
func (q *PrefixQuery) Boost(boost float32) *PrefixQuery {
	q.options["boost"] = boost
	return q
}

// MatchAllQuery represents a FTS match all query.
type MatchAllQuery struct {
}

// NewMatchAllQuery creates a new MatchAllQuery.
func NewMatchAllQuery(prefix string) *MatchAllQuery {
	return &MatchAllQuery{}
}

// MatchNoneQuery represents a FTS match none query.
type MatchNoneQuery struct {
}

// NewMatchNoneQuery creates a new MatchNoneQuery.
func NewMatchNoneQuery(prefix string) *MatchNoneQuery {
	return &MatchNoneQuery{}
}
