// Package types defines types that are used throughout the
// service, primarily struct types with JSON mappings and
// custom types used within those types, that are used for
// REST requests and respones.
package types

import (
	"bytes"
	"fmt"
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
	Code      string `json:"code"`
	Name      string `json:"name"`
	UnitPrice USD    `json:"unit_price"`
}

// ProduceAddRequest defines the JSON format for the request to add
// one or more items to the list of produce.
type ProduceAddRequest struct {
	Items []Produce `json:"items"`
}

// ProduceListResponse defines the JSON format for the request to list
// all of the produce items.  It is identical to the add request, but
// defined as a separate type for clarity.
type ProduceListResponse struct {
	Items []Produce `json:"items"`
}

// ProduceAddItemResponse is the repsonse to a single Produce add request.
// It contains the produce code and the HTTP status for a single add
// operation.  This is useful in the case of a partial success,
// so we can see exactly which ones succeeded and failed.
type ProduceAddItemResponse struct {
	Code       string `json:"code"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
}

// ProduceAddResponse is the repsonse to a Produce add request.  It
// is an array of items with the produce code and the HTTP status for
// that operation.
type ProduceAddResponse struct {
	Items []ProduceAddItemResponse `json:"items"`
}

// StatusResponse is the JSON returned for a liveness check as well as
// for other status notifications such as a successful delete.
type StatusResponse struct {
	Status string `json:"status"`
}

// ValidateAndConvertProduceCode returns whether the produce code is
// syntactically valid and if so, puts it in canoncial for (upper case).
func ValidateAndConvertProduceCode(code string) (string, bool) {
	if !codeExp.Match([]byte(code)) {
		return code, false
	}
	return strings.ToUpper(code), true
}

// ValidateAndConvertName returns whether the produce name is
// syntactically valid and if so, puts it in canoncial form.  For
// names, the canonical form is leading characters capitalized.  Also
// note, the leading character cannot bne a space, but internal characters
// may be white space.
func ValidateAndConvertName(name string) (string, bool) {
	if !nameExp.Match([]byte(name)) {
		return name, false
	}

	var prev = ' '
	runes := []rune(name)
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

// ValidateAndConvertProduce validates that the code and name comform
// to the grammar, and also canonicalize them as per the specified rules.
func ValidateAndConvertProduce(item *Produce) string {
	// The custom unmarshal of the USD field already validated it, but
	// we must manually validate the other two fields and convert
	// the to canonical format (upper case).
	var problems bytes.Buffer
	str, val := ValidateAndConvertProduceCode(item.Code)
	if !val {
		if problems.Len() != 0 {
			problems.WriteString(", ")
		}
		problems.WriteString(fmt.Sprintf("invalid code: '%s'", item.Code))
	}
	item.Code = str

	str, val = ValidateAndConvertName(item.Name)
	if !val {
		if problems.Len() != 0 {
			problems.WriteString(", ")
		}
		problems.WriteString(fmt.Sprintf("invalid name: '%s'", item.Name))
	}
	item.Name = str
	return problems.String()
}
