package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/ad3n/gonwrfc/gorfc"
)

func parseInput(input string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	var params map[string]any
	if err := decoder.Decode(&params); err != nil {
		return nil, fmt.Errorf("invalid params JSON: %w", err)
	}

	if params == nil {
		return nil, fmt.Errorf("params must be a JSON object")
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("params must contain exactly one JSON object")
	}

	return params, nil
}

func normalizeInput(params map[string]any, description gorfc.FunctionDescription) error {
	for _, parameter := range description.Parameters {
		value, ok := params[parameter.Name]
		if !ok {
			continue
		}

		normalized, err := normalizeValue(value, parameter.ParameterType, parameter.TypeDesc, parameter.Name)
		if err != nil {
			return err
		}

		params[parameter.Name] = normalized
	}

	return nil
}

func normalizeValue(value any, kind string, description gorfc.TypeDescription, path string) (any, error) {
	kind = strings.TrimPrefix(kind, "RFCTYPE_")
	switch kind {
	case "STRUCTURE":
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: expected an object", path)
		}

		for _, field := range description.Fields {
			value, ok := object[field.Name]
			if !ok {
				continue
			}

			normalized, err := normalizeValue(value, field.FieldType, field.TypeDesc, path+"."+field.Name)
			if err != nil {
				return nil, err
			}

			object[field.Name] = normalized
		}

		return object, nil
	case "TABLE":
		rows, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("%s: expected an array of objects", path)
		}

		for i, row := range rows {
			normalized, err := normalizeValue(row, "STRUCTURE", description, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}

			rows[i] = normalized
		}

		return rows, nil
	}

	number, ok := value.(json.Number)
	if !ok {
		return value, nil
	}

	switch kind {
	case "INT", "INT1", "INT2", "INT8":
		n, err := number.Int64()
		if err != nil {
			return nil, fmt.Errorf("%s: %s requires an integer JSON literal, got %s", path, kind, number)
		}

		minimum, maximum := int64(math.MinInt32), int64(math.MaxInt32)
		switch kind {
		case "INT1":
			minimum, maximum = 0, math.MaxUint8
		case "INT2":
			minimum, maximum = math.MinInt16, math.MaxInt16
		}

		if n < minimum || n > maximum {
			if kind == "INT8" {
				return nil, fmt.Errorf("%s: gonwrfc v1.0.11 writes INT8 through a 32-bit setter; refusing value %s", path, number)
			}

			return nil, fmt.Errorf("%s: %s must be between %d and %d, got %s", path, kind, minimum, maximum, number)
		}

		return int(n), nil
	case "FLOAT":
		n, err := number.Float64()
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return nil, fmt.Errorf("%s: FLOAT value %s is out of range", path, number)
		}

		return n, nil
	default:
		return number.String(), nil
	}
}
