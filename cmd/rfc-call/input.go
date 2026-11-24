package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
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

func validateInput(value any, schema map[string]any, path string) error {
	switch schema["type"] {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected an object", path)
		}

		properties, _ := schema["properties"].(map[string]any)
		for name, value := range object {
			field, ok := properties[name].(map[string]any)
			if !ok {
				continue
			}

			fieldPath := name
			if path != "" {
				fieldPath = path + "." + name
			}

			if err := validateInput(value, field, fieldPath); err != nil {
				return err
			}
		}
	case "array":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected an array", path)
		}

		item, _ := schema["items"].(map[string]any)
		for i, row := range rows {
			if err := validateInput(row, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			break
		}

		n, err := number.Int64()
		if err != nil || n < math.MinInt32 || n > math.MaxInt32 {
			return fmt.Errorf("%s: expected a signed 32-bit integer, got %s", path, number)
		}
	}

	return nil
}
