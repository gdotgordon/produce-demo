package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type Fubar struct {
	Bucks USD `json:"bucks"`
}

func TestUSDConversion(t *testing.T) {
	for i, v := range []struct {
		input string
		uerr  string
		value USD
		mstr  string
	}{
		{
			input: "$3.25",
			value: USD(325),
			mstr:  "$3.25",
		},
		{
			input: "3.25",
			value: USD(325),
			mstr:  "$3.25",
		},
		{
			input: "$3",
			value: USD(300),
			mstr:  "$3.00",
		},
		{
			input: "3",
			value: USD(300),
			mstr:  "$3.00",
		},
		{
			input: "0.72",
			value: USD(72),
			mstr:  "$0.72",
		},
		{
			input: "0.1",
			value: USD(10),
			mstr:  "$0.10",
		},
		{
			input: "$0.00",
			value: USD(0),
			mstr:  "$0.00",
		},
		{
			input: "$3.256",
			uerr:  "invalid USD format",
		},
		{
			input: "$",
			uerr:  "invalid USD format",
		},
		{
			input: "-$4.56",
			uerr:  "invalid USD format",
		},
	} {
		var f Fubar
		fstr := []byte(fmt.Sprintf(`{"bucks": "%s"}`, v.input))
		err := json.Unmarshal(fstr, &f)
		if err != nil {
			if v.uerr == "" {
				t.Fatalf("(%d) couldn't unmarshal %s", i, v.input)
			} else if !strings.Contains(err.Error(), v.uerr) {
				t.Fatalf("(%d) got unexpected error message %s", i, err.Error())
			}
		}
		if v.uerr != "" {
			continue
		}
		if f.Bucks != v.value {
			t.Fatalf("(%d) unexpected unamrashal value: %s", i, f.Bucks)
		}
		b, err := json.Marshal(f)
		if err != nil {
			t.Fatalf("(%d) couldn't marshal %s", i, f.Bucks)
		}
		exp := fmt.Sprintf(`"bucks":"%s"`, v.mstr)
		if !strings.Contains(string(b), exp) {
			t.Fatalf("(%d) Marshal did not contain '%s'", i, v.mstr)
		}
	}

}
