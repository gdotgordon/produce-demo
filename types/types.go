// Package types defines types that are used throughout the
// service, primarily struct types with JSON mappings and
// custom types used within those types, that are used for
// REST requests and respones.
package types

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// Regular expression to validate a produce code, which is 4 sets of
	// hyphen-separated quartets of alphanumerics.
	codeExp = regexp.MustCompile(`^([A-Za-z0-9]{4}-){3}([A-Za-z0-9]){4}$`)

	// Regular expression to match produce name: (Unicode) alphanumerics
	// plus white space.
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

// StatusResponse is the JSON returned for a liveness check.
type StatusResponse struct {
	Status string `json:"status"`
}

// ValidateAndConvertProduceCode returns whether the produce code is
// syntactically valid and if so, puts it in canoncial for (upper case).
func ValidateAndConvertProduceCode(code string) (string, bool) {
	if !codeExp.Match([]byte(code)) {
		return "", false
	}
	return strings.ToUpper(code), true
}

// ValidateAndConvertName returns whether the produce name is
// syntactically valid and if so, puts it in canoncial form.  For
// names, the canonical form is leading characters capitalized.  Also
// note, the leading character cannot bne a space, but internal characters
// may be white space.
func ValidateAndConvertName(code string) (string, bool) {
	if !nameExp.Match([]byte(code)) {
		return "", false
	}

	var prev = ' '
	runes := []rune(code)
	var res []rune
	for _, v := range runes {
		if unicode.IsSpace(prev) {
			res = append(res, unicode.ToUpper(v))
		} else {
			res = append(res, unicode.ToLower(v))
		}
		prev = v
	}
	return string(res), true
}
