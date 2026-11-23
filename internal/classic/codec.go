package classic

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ad3n/sap-rfc-classic-mock/internal/config"
)

type layout struct {
	offsets []int
	size    int
}

func geometry(fields []config.Field, unicode bool) layout {
	result := layout{offsets: make([]int, len(fields))}
	maximum := 1
	for i, f := range fields {
		width, alignment := dimensions(f.Parameter(), unicode)
		maximum = max(maximum, alignment)
		result.size = aligned(result.size, alignment)
		result.offsets[i] = result.size
		result.size += width
	}

	result.size = aligned(result.size, maximum)
	return result
}

func aligned(n, alignment int) int { return (n + alignment - 1) / alignment * alignment }

func dimensions(p config.Parameter, unicode bool) (int, int) {
	switch p.Type {
	case "CHAR":
		if unicode {
			return p.Length * 2, 2
		}

		return p.Length, 1
	case "INT":
		return 4, 4
	case "FLOAT", "STRING":
		return 8, 8
	case "STRUCTURE", "TABLE":
		l := geometry(p.Fields, unicode)
		return l.size, 1
	}

	return 0, 1
}

func chars(s string, width int) []byte {
	units := utf16.Encode([]rune(s))
	result := make([]byte, width*2)
	for i := range width {
		unit := uint16(' ')
		if i < len(units) {
			unit = units[i]
		}

		binary.LittleEndian.PutUint16(result[i*2:], unit)
	}

	return result
}

func integer(n int) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, uint32(n))
	return result
}

func decode(p config.Parameter, data []byte, order binary.ByteOrder) (any, error) {
	if order == nil {
		order = binary.LittleEndian
	}

	width, _ := dimensions(p, true)
	if p.Type != "STRING" && len(data) != width {
		return nil, fmt.Errorf("%s: received %d bytes, expected %d", p.Name, len(data), width)
	}

	switch p.Type {
	case "STRING":
		if len(data) == 0 || data[len(data)-1] != 0 || !utf8.Valid(data[:len(data)-1]) {
			return nil, fmt.Errorf("%s: invalid RFC STRING", p.Name)
		}

		return string(data[:len(data)-1]), nil
	case "CHAR":
		if len(data)%2 != 0 {
			return nil, fmt.Errorf("%s: invalid UTF-16 length", p.Name)
		}

		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = order.Uint16(data[i*2:])
		}

		text := string(utf16.Decode(units))
		if p.Type == "CHAR" {
			text = strings.TrimRight(text, " ")
		}

		return text, nil
	case "INT":
		return int(int32(order.Uint32(data))), nil
	case "FLOAT":
		return math.Float64frombits(order.Uint64(data)), nil
	case "STRUCTURE":
		result := make(map[string]any, len(p.Fields))
		l := geometry(p.Fields, true)
		for i, f := range p.Fields {
			width, _ := dimensions(f.Parameter(), true)
			value, err := decode(f.Parameter(), data[l.offsets[i]:l.offsets[i]+width], order)
			if err != nil {
				return nil, err
			}

			result[f.Name] = value
		}

		return result, nil
	}

	return nil, fmt.Errorf("unsupported decode type %s", p.Type)
}

func encode(p config.Parameter, value any) ([]byte, error) {
	if value == nil {
		value = initial(p)
	}

	switch p.Type {
	case "CHAR":
		return chars(value.(string), p.Length), nil
	case "STRING":
		return append([]byte(value.(string)), 0), nil
	case "INT":
		return integer(value.(int)), nil
	case "FLOAT":
		number, ok := value.(float64)
		if !ok {
			number = float64(value.(int))
		}

		result := make([]byte, 8)
		binary.LittleEndian.PutUint64(result, math.Float64bits(number))
		return result, nil
	case "STRUCTURE":
		l := geometry(p.Fields, true)
		result := make([]byte, l.size)
		object := value.(map[string]any)
		for i, f := range p.Fields {
			bytes, err := encode(f.Parameter(), object[f.Name])
			if err != nil {
				return nil, err
			}

			copy(result[l.offsets[i]:], bytes)
		}

		return result, nil
	}

	return nil, fmt.Errorf("unsupported encode type %s", p.Type)
}

func initial(p config.Parameter) any {
	switch p.Type {
	case "INT":
		return 0
	case "FLOAT":
		return float64(0)
	case "STRUCTURE":
		return map[string]any{}
	case "TABLE":
		return []any{}
	default:
		return ""
	}
}
