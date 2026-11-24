package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateJSONInput(t *testing.T) {
	integer := map[string]any{"type": "integer"}
	row := map[string]any{"type": "object", "properties": map[string]any{"QUANTITY": integer}}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"CV_COUNTER": integer,
			"IS_HEADER":  row,
			"T_ECHO":     map[string]any{"type": "array", "items": row},
		},
	}

	for _, test := range []struct {
		input string
		path  string
	}{
		{input: `{"CV_COUNTER":42,"IS_HEADER":{"QUANTITY":-2},"T_ECHO":[{"QUANTITY":2147483647},{"QUANTITY":-2147483648}]}`},
		{input: `{"CV_COUNTER":2147483648}`, path: "CV_COUNTER"},
		{input: `{"CV_COUNTER":-2147483649}`, path: "CV_COUNTER"},
		{input: `{"CV_COUNTER":9007199254740993}`, path: "CV_COUNTER"},
		{input: `{"CV_COUNTER":1.5}`, path: "CV_COUNTER"},
		{input: `{"CV_COUNTER":1.0000000000000000001}`, path: "CV_COUNTER"},
		{input: `{"CV_COUNTER":1e2}`, path: "CV_COUNTER"},
		{input: `{"IS_HEADER":null}`, path: "IS_HEADER"},
		{input: `{"T_ECHO":null}`, path: "T_ECHO"},
		{input: `{"T_ECHO":[42]}`, path: "T_ECHO[0]"},
		{input: `{"T_ECHO":[{"QUANTITY":2147483648}]}`, path: "T_ECHO[0].QUANTITY"},
	} {
		t.Run(test.input, func(t *testing.T) {
			params, err := parseInput(test.input)
			if err != nil {
				t.Fatal(err)
			}

			err = validateInput(params, schema, "")
			if test.path == "" {
				if err != nil {
					t.Fatal(err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), test.path+":") {
				t.Fatalf("expected error at %s, got %v", test.path, err)
			}
		})
	}
}

func TestPreserveJSONNumberPrecision(t *testing.T) {
	params, err := parseInput(`{"VALUE":9007199254740993.123456789}`)
	if err != nil {
		t.Fatal(err)
	}

	if got := params["VALUE"]; got != json.Number("9007199254740993.123456789") {
		t.Fatalf("VALUE = %#v", got)
	}
}

func TestInvalidJSONInput(t *testing.T) {
	for _, input := range []string{"null", "[]", "42", `{} {}`, `{} trailing`, `{"X":`} {
		if _, err := parseInput(input); err == nil {
			t.Fatalf("accepted invalid input %s", input)
		}
	}
}
