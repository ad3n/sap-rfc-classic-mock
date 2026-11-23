package classic

import (
	"context"
	"testing"

	wire "github.com/ad3n/open-rfc-go/mockbridge"
	"github.com/ad3n/sap-rfc-classic-mock/internal/config"
)

func benchmarkRow() (config.Parameter, map[string]any) {
	return config.Parameter{Type: "STRUCTURE", Fields: []config.Field{
		{Name: "CODE", Type: "CHAR", Length: 12},
		{Name: "QUANTITY", Type: "INT"},
		{Name: "PRICE", Type: "FLOAT"},
	}}, map[string]any{"CODE": "Lawang世界😀", "QUANTITY": -7, "PRICE": 1.25}
}

func BenchmarkEncodeStructure(b *testing.B) {
	p, value := benchmarkRow()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := encode(p, value); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandlerTable128(b *testing.B) {
	p, value := benchmarkRow()
	data, err := encode(p, value)
	if err != nil {
		b.Fatal(err)
	}
	p.Name, p.Type, p.Direction = "T_DATA", "TABLE", "tables"
	rows := make([][]byte, 128)
	for i := range rows {
		rows[i] = data
	}
	handler := functionHandler(config.Function{Parameters: []config.Parameter{p}})
	req := wire.Request{Tables: []wire.Table{{Name: p.Name, Rows: rows}}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := handler(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandlerScalar(b *testing.B) {
	handler := functionHandler(config.Function{Parameters: []config.Parameter{
		{Name: "INPUT", Type: "CHAR", Length: 40, Direction: "import"},
		{Name: "OUTPUT", Type: "CHAR", Length: 40, Direction: "export", From: "INPUT"},
	}})
	req := wire.Request{Imports: []wire.NamedValue{{Name: "INPUT", Value: chars("Hello Lawang", 40)}}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := handler(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeStructureInto(b *testing.B) {
	p, value := benchmarkRow()
	width, _ := dimensions(p, true)
	dst := make([]byte, width)
	b.ReportAllocs()
	for b.Loop() {
		if err := encodeInto(dst, p, value); err != nil {
			b.Fatal(err)
		}
	}
}
