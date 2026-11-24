package config

import (
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"regexp"

	"gopkg.in/yaml.v3"
)

var namePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,29}$`)

type (
	Config struct {
		Transport  string     `yaml:"transport,omitempty"`
		ListenHost string     `yaml:"listen_host,omitempty"`
		Functions  []Function `yaml:"functions"`
		Port       int        `yaml:"port"`
	}

	Function struct {
		Name       string      `yaml:"name"`
		Parameters []Parameter `yaml:"parameters"`
	}

	Parameter struct {
		Value     any     `yaml:"value"`
		Name      string  `yaml:"name"`
		Type      string  `yaml:"type"`
		Direction string  `yaml:"direction"`
		From      string  `yaml:"from"`
		Fields    []Field `yaml:"fields,omitempty"`
		Length    int     `yaml:"length"`
	}

	Field struct {
		Name   string  `yaml:"name"`
		Type   string  `yaml:"type"`
		Fields []Field `yaml:"fields,omitempty"`
		Length int     `yaml:"length,omitempty"`
	}
)

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("config must contain exactly one YAML document")
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Transport != "" && c.Transport != "classic" {
		return fmt.Errorf("transport must be classic")
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if len(c.Functions) == 0 {
		return fmt.Errorf("functions cannot be empty")
	}

	names := make(map[string]bool)
	for _, fn := range c.Functions {
		if !namePattern.MatchString(fn.Name) || names[fn.Name] || fn.Name == "RFC_GET_FUNCTION_INTERFACE" || fn.Name == "RFC_GET_STRUCTURE_DEFINITION" || fn.Name == "RFC_PING" || fn.Name == "DDIF_FIELDINFO_GET" {
			return fmt.Errorf("invalid or duplicate function %q", fn.Name)
		}

		names[fn.Name] = true
		params := make(map[string]Parameter)
		for _, p := range fn.Parameters {
			if !namePattern.MatchString(p.Name) {
				return fmt.Errorf("%s: invalid parameter %q", fn.Name, p.Name)
			}

			if _, ok := params[p.Name]; ok {
				return fmt.Errorf("%s: duplicate parameter %s", fn.Name, p.Name)
			}

			params[p.Name] = p
			switch p.Direction {
			case "import", "export", "changing", "tables":
			default:
				return fmt.Errorf("%s.%s: direction must be import, export, changing, or tables", fn.Name, p.Name)
			}

			if err := validateField(Field{Name: p.Name, Type: p.Type, Length: p.Length, Fields: p.Fields}, 0); err != nil {
				return fmt.Errorf("%s.%s: %w", fn.Name, p.Name, err)
			}
			if p.Type == "TABLE" && p.Direction != "tables" {
				return fmt.Errorf("%s.%s: TABLE requires tables direction", fn.Name, p.Name)
			}

			if p.Direction == "tables" && p.Type != "TABLE" {
				return fmt.Errorf("%s.%s: tables direction requires TABLE", fn.Name, p.Name)
			}

			if p.Direction == "import" && (p.Value != nil || p.From != "") {
				return fmt.Errorf("%s.%s: import cannot have value or from", fn.Name, p.Name)
			}

			if p.Value != nil && p.From != "" {
				return fmt.Errorf("%s.%s: use either value or from", fn.Name, p.Name)
			}

			if p.Value != nil {
				if err := p.ValidateValue(p.Value); err != nil {
					return fmt.Errorf("%s.%s: %w", fn.Name, p.Name, err)
				}
			}
		}

		for _, p := range fn.Parameters {
			if p.From == "" {
				continue
			}

			src, ok := params[p.From]
			if !ok || src.Direction == "export" || src.Type != p.Type || src.Length != p.Length || !reflect.DeepEqual(src.Fields, p.Fields) {
				return fmt.Errorf("%s.%s: from must reference an import/changing/tables parameter of the same type and length, including fields", fn.Name, p.Name)
			}
		}
	}

	return nil
}

func (p Parameter) ValidateValue(value any) error {
	switch p.Type {
	case "STRUCTURE":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("value must be an object")
		}

		fields := make(map[string]Field, len(p.Fields))
		for _, field := range p.Fields {
			fields[field.Name] = field
		}

		for key, v := range object {
			field, ok := fields[key]
			if !ok {
				return fmt.Errorf("unknown field %s", key)
			}

			if v == nil {
				return fmt.Errorf("field %s cannot be null", key)
			}

			if err := field.Parameter().ValidateValue(v); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
	case "TABLE":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("value must be a list of objects")
		}

		row := Parameter{Type: "STRUCTURE", Fields: p.Fields}
		for i, value := range rows {
			if err := row.ValidateValue(value); err != nil {
				return fmt.Errorf("row %d: %w", i, err)
			}
		}
	case "CHAR", "STRING":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("value must be a string")
		}

		units := 0
		for _, r := range s {
			units++
			if r > 0xffff {
				units++
			}
		}

		if p.Type == "CHAR" && units > p.Length {
			return fmt.Errorf("value exceeds CHAR length")
		}
	case "INT":
		n, ok := value.(int)
		if !ok || int64(n) < -2147483648 || int64(n) > 2147483647 {
			return fmt.Errorf("value must be a signed 32-bit integer")
		}
	case "FLOAT":
		switch value.(type) {
		case int:
		case float64:
			n := value.(float64)
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return fmt.Errorf("value must be finite")
			}
		default:
			return fmt.Errorf("value must be numeric")
		}
	}

	return nil
}

func (f Field) Parameter() Parameter {
	return Parameter{Name: f.Name, Type: f.Type, Length: f.Length, Fields: f.Fields}
}

func validateField(f Field, depth int) error {
	if depth > 8 {
		return fmt.Errorf("field nesting exceeds 8 levels")
	}

	if !namePattern.MatchString(f.Name) {
		return fmt.Errorf("invalid field name %q", f.Name)
	}

	switch f.Type {
	case "STRUCTURE", "TABLE":
		if f.Length != 0 || len(f.Fields) == 0 {
			return fmt.Errorf("%s requires fields and cannot set length", f.Type)
		}

		seen := make(map[string]bool)
		for _, field := range f.Fields {
			if field.Type == "STRUCTURE" || field.Type == "TABLE" || field.Type == "STRING" {
				return fmt.Errorf("field %s: classic composite fields support CHAR, INT, FLOAT only", field.Name)
			}

			if seen[field.Name] {
				return fmt.Errorf("duplicate field %s", field.Name)
			}

			seen[field.Name] = true
			if err := validateField(field, depth+1); err != nil {
				return fmt.Errorf("%s: %w", field.Name, err)
			}
		}
	case "CHAR":
		if f.Length < 1 || f.Length > 65535 {
			return fmt.Errorf("CHAR length must be 1..65535")
		}
	case "STRING", "INT", "FLOAT":
		if f.Length != 0 {
			return fmt.Errorf("length is only valid for CHAR")
		}
	default:
		return fmt.Errorf("unsupported type %q", f.Type)
	}

	if f.Type != "STRUCTURE" && f.Type != "TABLE" && len(f.Fields) != 0 {
		return fmt.Errorf("scalar cannot have fields")
	}

	return nil
}
