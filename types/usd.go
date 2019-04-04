package types

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// The usd regex represents currency with an optional leading '$'.
	// Integer numbers without decimal points are valid, as are
	// numbers with one or two digits after the decimal point.  Note
	// as the per the JSON spec (http://www.json.org), any fractional
	// part with no whole number part must strt with as '0', i.e. "0.7"
	// and not ".7".
	// https://www.regular-expressions.info/unicode.html#prop
	usdExp = regexp.MustCompile(`^\"\$?\d*.(\.\d{1,2})?\"$`)
)

// USD represents US Dollars by storing the total number of cents as an
// unsigned int 32.  This is in fact the approach Stripe uses to store
// currency.  Note, the user specifies the JSON as a string, but internally
// we store it in our format using custom JSON un(marshalers.)
type USD uint32

// String is the Stringer() interface implementation.
func (d USD) String() string {
	return fmt.Sprintf("$%d.%02d", d/100, d%100)
}

// UnmarshalJSON is a custom JSON unmarshaller for USD currency.
func (d *USD) UnmarshalJSON(b []byte) error {
	if !usdExp.Match(b) {
		return errors.New("invalid USD format: " + string(b))
	}

	// Strip surrounding quotes and any leading dollar sign
	b = b[1 : len(b)-1]
	if b[0] == '$' {
		b = b[1:]
	}

	// Parse the remaining parts.
	str := string(b)
	var value uint32
	var n uint64
	var err error
	ndx := strings.Index(str, ".")
	if ndx == -1 {
		n, err = strconv.ParseUint(str, 10, 32)
		if err != nil {
			return errors.New("invalid USD format: " + string(b))
		}
		value = 100 * uint32(n)
	} else {
		n, err = strconv.ParseUint(str[:ndx], 10, 32)
		if err != nil {
			return errors.New("invalid USD format: " + string(b))
		}
		value = 100 * uint32(n)
		frac := str[ndx+1:]
		n, err = strconv.ParseUint(str[ndx+1:], 10, 32)
		if err != nil {
			return errors.New("invalid USD format: " + string(b))
		}
		if len(frac) == 1 {
			value += 10 * uint32(n)
		} else {
			value += uint32(n)
		}
	}
	*d = USD(value)
	return nil
}

// MarshalJSON is a custom JSON marshaller for USD currency.
func (d USD) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('"')
	buf.WriteString(d.String())
	buf.WriteByte('"')
	return buf.Bytes(), nil
}
