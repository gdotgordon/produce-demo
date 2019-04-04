// Package types defines types that are used throughout the
// service, primarily struct types with JSON mappings and
// custom types used within those types, that are used for
// REST requests and respones.
package types

import "regexp"

var (
	// Regular expression to validate a produce code.  Note p{L} matches
	/// all Unicode alphas and p{N} all numerics.
	codeExp = regexp.MustCompile(`^([\p{L}\p{N}]{4}-){3}[\p{L}\p{N}]{4}$`)

	// Regular expression to match produce name: alphanumerics plus white space
	nameExp = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}\s]*$`)
)

// Produce represents a code, name and unit price for an item in
// the supermarket.  Note the unit price is a custom type that maps
// as JSON string to an internal format that can be worked with
// mathematically.
type Produce struct {
	ProduceCode string `json:"produce_code"`
	Name        string `json:"name"`
	UnitPrice   USD    `json:"unit_price"`
}

// ProduceAddRequest defines the JSON format for the request to add
// one or more items to the list of produce.
type ProduceAddRequest struct {
	Items []Produce `json:"items"`
}

// ProduceAddItemResponse is the repsonse to a single Produce add request.
// It contains the produce code and the HTTP status for a single add
// operation.  This is useful in the case of a partial success,
// so we can see exactly which ones succeeded and failed.
type ProduceAddItemResponse struct {
	ProduceCode string `json:"produce_code"`
	StatusCode  int    `json:"status_code"`
}

// ProduceAddResponse is the repsonse to a Produce add request.  It
// is an array of items with the produce code and the HTTP status for
// that operation.
type ProduceAddResponse struct {
	Items []ProduceAddItemResponse `json:"items"`
}
