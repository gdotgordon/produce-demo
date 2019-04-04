package types

import "testing"

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
