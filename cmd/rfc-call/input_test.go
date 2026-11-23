package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ad3n/gonwrfc/gorfc"
)

func TestNormalizeJSONInput(t *testing.T) {
	fields := gorfc.TypeDescription{Fields: []gorfc.FieldDescription{
		{Name: "QUANTITY", FieldType: "RFCTYPE_INT"},
		{Name: "PRICE", FieldType: "RFCTYPE_FLOAT"},
		{Name: "CODE", FieldType: "RFCTYPE_CHAR"},
	}}
	description := gorfc.FunctionDescription{Parameters: []gorfc.ParameterDescription{
		{Name: "CV_COUNTER", ParameterType: "RFCTYPE_INT"},
		{Name: "IS_HEADER", ParameterType: "RFCTYPE_STRUCTURE", TypeDesc: fields},
		{Name: "T_ECHO", ParameterType: "RFCTYPE_TABLE", TypeDesc: fields},
	}}
	params, err := parseInput(`{"CV_COUNTER":42,"IS_HEADER":{"QUANTITY":-2,"PRICE":12.5,"CODE":"0002"},"T_ECHO":[{"QUANTITY":2147483647,"PRICE":7},{"QUANTITY":-2147483648,"PRICE":1.25e2}]}`)
	if err != nil {
		t.Fatal(err)
	}

	if err := normalizeInput(params, description); err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"CV_COUNTER": int(42),
		"IS_HEADER":  map[string]any{"QUANTITY": int(-2), "PRICE": float64(12.5), "CODE": "0002"},
		"T_ECHO":     []any{map[string]any{"QUANTITY": int(2147483647), "PRICE": float64(7)}, map[string]any{"QUANTITY": int(-2147483648), "PRICE": float64(125)}},
	}
	if !reflect.DeepEqual(params, expected) {
		t.Fatalf("unexpected converted input: %#v", params)
	}
}

func TestNumberValidation(t *testing.T) {
	tests := []struct{ kind, value string }{
		{kind: "INT", value: "2147483648"},
		{kind: "INT", value: "-2147483649"},
		{kind: "INT", value: "9007199254740993"},
		{kind: "INT", value: "1.5"},
		{kind: "INT", value: "1.0000000000000000001"},
		{kind: "INT", value: "1e2"},
		{kind: "INT1", value: "256"},
		{kind: "INT1", value: "-1"},
		{kind: "INT2", value: "32768"},
		{kind: "INT8", value: "2147483648"},
		{kind: "FLOAT", value: "1e400"},
	}
	for _, test := range tests {
		t.Run(test.kind+"/"+test.value, func(t *testing.T) {
			_, err := normalizeValue(json.Number(test.value), test.kind, gorfc.TypeDescription{}, "T_ITEMS[2].QUANTITY")
			if err == nil || !strings.Contains(err.Error(), "T_ITEMS[2].QUANTITY") {
				t.Fatalf("expected contextual error, got %v", err)
			}
		})
	}
}

func TestPreserveTextAndDecimalPrecision(t *testing.T) {
	for _, kind := range []string{"CHAR", "STRING", "BCD", "DECF34"} {
		got, err := normalizeValue(json.Number("9007199254740993.123456789"), kind, gorfc.TypeDescription{}, "VALUE")
		if err != nil || got != "9007199254740993.123456789" {
			t.Fatalf("%s: %v, %v", kind, got, err)
		}
	}
}

func TestInvalidJSONInput(t *testing.T) {
	for _, input := range []string{"null", "[]", "42", `{} {}`, `{} trailing`, `{"X":`} {
		if _, err := parseInput(input); err == nil {
			t.Fatalf("accepted invalid input %s", input)
		}
	}
}

func TestInvalidCompositeInput(t *testing.T) {
	for _, kind := range []string{"STRUCTURE", "TABLE"} {
		if _, err := normalizeValue(nil, kind, gorfc.TypeDescription{}, "PARAM"); err == nil {
			t.Fatalf("accepted null %s", kind)
		}
	}

	if _, err := normalizeValue([]any{42}, "TABLE", gorfc.TypeDescription{}, "ROWS"); err == nil || !strings.Contains(err.Error(), "ROWS[0]") {
		t.Fatalf("unexpected error %v", err)
	}
}
