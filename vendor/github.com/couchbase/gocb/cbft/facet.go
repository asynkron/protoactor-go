package cbft

import (
	"encoding/json"
)

// *VOLATILE*
// FtsFacet represents a facet for a search query.
type FtsFacet interface {
}

type termFacetData struct {
	Field string `json:"field,omitempty"`
	Size  int    `json:"size,omitempty"`
}

// TermFacet is an FTS term facet.
type TermFacet struct {
	data termFacetData
}

// MarshalJSON marshal's this facet to JSON for the FTS REST API.
func (f TermFacet) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.data)
}

// NewTermFacet creates a new TermFacet
func NewTermFacet(field string, size int) *TermFacet {
	mq := &TermFacet{}
	mq.data.Field = field
	mq.data.Size = size
	return mq
}

type numericFacetRange struct {
	Name  string  `json:"name,omitempty"`
	Start float64 `json:"start,omitempty"`
	End   float64 `json:"end,omitempty"`
}
type numericFacetData struct {
	Field         string              `json:"field,omitempty"`
	Size          int                 `json:"size,omitempty"`
	NumericRanges []numericFacetRange `json:"numeric_ranges,omitempty"`
}

// NumericFacet is an FTS numeric range facet.
type NumericFacet struct {
	data numericFacetData
}

// MarshalJSON marshal's this facet to JSON for the FTS REST API.
func (f NumericFacet) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.data)
}

// AddRange adds a new range to this numeric range facet.
func (f *NumericFacet) AddRange(name string, start, end float64) *NumericFacet {
	f.data.NumericRanges = append(f.data.NumericRanges, numericFacetRange{
		Name:  name,
		Start: start,
		End:   end,
	})
	return f
}

// NewNumericFacet creates a new numeric range facet.
func NewNumericFacet(field string, size int) *NumericFacet {
	mq := &NumericFacet{}
	mq.data.Field = field
	mq.data.Size = size
	return mq
}

type dateFacetRange struct {
	Name  string `json:"name,omitempty"`
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}
type dateFacetData struct {
	Field      string           `json:"field,omitempty"`
	Size       int              `json:"size,omitempty"`
	DateRanges []dateFacetRange `json:"date_ranges,omitempty"`
}

// DateFacet is an FTS date range facet.
type DateFacet struct {
	data dateFacetData
}

// MarshalJSON marshal's this facet to JSON for the FTS REST API.
func (f DateFacet) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.data)
}

// AddRange adds a new range to this date range facet.
func (f *DateFacet) AddRange(name string, start, end string) *DateFacet {
	f.data.DateRanges = append(f.data.DateRanges, dateFacetRange{
		Name:  name,
		Start: start,
		End:   end,
	})
	return f
}

// NewDateFacet creates a new date range facet.
func NewDateFacet(field string, size int) *DateFacet {
	mq := &DateFacet{}
	mq.data.Field = field
	mq.data.Size = size
	return mq
}
