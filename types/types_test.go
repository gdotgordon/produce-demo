package types

import (
	"testing"
)

var (
	dfltProduce = Produce{
		Code:      "A12T-4GH7-QPL9-3N4M",
		Name:      "Lettuce",
		UnitPrice: (346),
	}

	secondProduce = Produce{
		Code:      "YRT6-72AS-K736-L4AR",
		Name:      "Green Pepper",
		UnitPrice: (79),
	}

	dfltLCProduce = Produce{
		Code:      "a12t-4gh7-qpL9-3n4m",
		Name:      "lettuce",
		UnitPrice: (346),
	}

	dfltProduceBadCode = Produce{
		Code:      "A12T-4GH7-QP",
		Name:      "Lettuce",
		UnitPrice: (346),
	}

	dfltProduceBadName = Produce{
		Code:      "A12T-4GH7-QPL9-3N4M",
		Name:      "Lettuce+Cukes",
		UnitPrice: (346),
	}

	noProduce = Produce{}
)

func TestProduceCodeConversion(t *testing.T) {
	for i, v := range []struct {
		input    string
		valid    bool
		expected string
	}{
		{
			input:    "TQ4C-VV6T-75ZX-1RMR",
			valid:    true,
			expected: "TQ4C-VV6T-75ZX-1RMR",
		},
		{
			input:    "Tq4C-VV6t-75ZX-1rMR",
			valid:    true,
			expected: "TQ4C-VV6T-75ZX-1RMR",
		},
		{
			input: "T%4C-VV6t-75ZX-1)MR",
			valid: false,
		},
		{
			input: "Tq4C-VV6t-75ZX",
			valid: false,
		},
		{
			input: "",
			valid: false,
		},
	} {
		str, valid := ValidateAndConvertProduceCode(v.input)
		if v.valid != valid {
			t.Fatalf("(%d) Unexpected validation result", i)
		}
		if str != v.expected {
			t.Fatalf("(%d) Unexpected converted string: '%s'", i, str)
		}
	}
}

func TestProduceNameConversion(t *testing.T) {
	for i, v := range []struct {
		input    string
		valid    bool
		expected string
	}{
		{
			input:    "Lettuce",
			valid:    true,
			expected: "Lettuce",
		},
		{
			input:    "Green Pepper",
			valid:    true,
			expected: "Green Pepper",
		},
		{
			input:    "Jalapeño",
			valid:    true,
			expected: "Jalapeño",
		},
		{
			input:    "jalapeño",
			valid:    true,
			expected: "Jalapeño",
		},
		{
			input:    "green pepper",
			valid:    true,
			expected: "Green Pepper",
		},
		{
			input:    "grEen pePper",
			valid:    true,
			expected: "Green Pepper",
		},
		{
			input:    "lettuce 2",
			valid:    true,
			expected: "Lettuce 2",
		},
		{
			input: " green pepper",
			valid: false,
		},
		{
			input: "",
			valid: false,
		},
	} {
		str, valid := ValidateAndConvertName(v.input)
		if v.valid != valid {
			t.Fatalf("(%d) Unexpected validation result", i)
		}
		if str != v.expected {
			t.Fatalf("(%d) Unexpected converted string: '%s'", i, str)
		}
	}
}

func TestProduceConversion(t *testing.T) {
	for i, v := range []struct {
		input   Produce
		expStr  string
		expProd Produce
	}{
		{
			input:   dfltProduce,
			expProd: dfltProduce,
		},
		{
			input:   dfltLCProduce,
			expProd: dfltProduce,
		},
		{
			input:  dfltProduceBadCode,
			expStr: "invalid code: 'A12T-4GH7-QP'",
		},
		{
			input:  dfltProduceBadName,
			expStr: "invalid name: 'Lettuce+Cukes'",
		},
	} {
		citem := v.input
		str := ValidateAndConvertProduce(&citem)
		if str != v.expStr {
			t.Fatalf("(%d) Unexpected converted string: '%s'", i, str)
		}
		if v.expProd != noProduce {
			if citem != v.expProd {
				t.Fatalf("(%d) Bad produce conversion: '%+v'", i, citem)
			}
		}
	}
}
