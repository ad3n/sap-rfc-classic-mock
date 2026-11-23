package classic

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"rfc-server/internal/config"
)

func decodeXML(p config.Parameter, data []byte) (any, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	root, ok := token.(xml.StartElement)
	if !ok || root.Name.Local != p.Name {
		return nil, fmt.Errorf("unexpected xRFC root for %s", p.Name)
	}

	value, err := xmlValue(decoder, root, p)
	if err != nil {
		return nil, err
	}

	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing xRFC data")
	}

	if err := p.ValidateValue(value); err != nil {
		return nil, err
	}

	return value, nil
}

func xmlValue(decoder *xml.Decoder, start xml.StartElement, p config.Parameter) (any, error) {
	if p.Type != "STRUCTURE" && p.Type != "TABLE" {
		var text string
		if err := decoder.DecodeElement(&text, &start); err != nil {
			return nil, err
		}

		switch p.Type {
		case "INT":
			n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 32)
			return int(n), err
		case "FLOAT":
			return strconv.ParseFloat(strings.TrimSpace(text), 64)
		case "CHAR":
			return strings.TrimRight(text, " "), nil
		default:
			return text, nil
		}
	}

	fields := make(map[string]config.Field)
	for _, field := range p.Fields {
		fields[field.Name] = field
	}

	object := make(map[string]any)
	rows := []any{}
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch child := token.(type) {
		case xml.EndElement:
			if p.Type == "TABLE" {
				return rows, nil
			}

			return object, nil
		case xml.StartElement:
			field, ok := fields[child.Name.Local]
			parameter := field.Parameter()
			if p.Type == "TABLE" {
				ok = child.Name.Local == "item"
				parameter = config.Parameter{Type: "STRUCTURE", Fields: p.Fields}
			}

			if !ok {
				return nil, fmt.Errorf("unknown xRFC field %s", child.Name.Local)
			}

			value, err := xmlValue(decoder, child, parameter)
			if err != nil {
				return nil, err
			}

			if p.Type == "TABLE" {
				rows = append(rows, value)
				continue
			}

			if _, exists := object[child.Name.Local]; exists {
				return nil, fmt.Errorf("duplicate xRFC field %s", child.Name.Local)
			}

			object[child.Name.Local] = value
		case xml.CharData:
			if strings.TrimSpace(string(child)) != "" {
				return nil, fmt.Errorf("unexpected xRFC text")
			}
		}
	}
}
