package classic

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/ad3n/sap-rfc-classic-mock/internal/config"
)

func TestAlignedStructure(t *testing.T) {
	p := config.Parameter{Name: "ROW", Type: "STRUCTURE", Fields: []config.Field{
		{Name: "CODE", Type: "CHAR", Length: 5},
		{Name: "QUANTITY", Type: "INT"},
		{Name: "PRICE", Type: "FLOAT"},
	}}
	if got := geometry(p.Fields, true); got.size != 24 || !reflect.DeepEqual(got.offsets, []int{0, 12, 16}) {
		t.Fatalf("unicode layout: %+v", got)
	}

	if got := geometry(p.Fields, false); got.size != 24 || !reflect.DeepEqual(got.offsets, []int{0, 8, 16}) {
		t.Fatalf("non-unicode layout: %+v", got)
	}

	want := map[string]any{"CODE": "世界", "QUANTITY": -7, "PRICE": float64(1.25)}
	encoded, err := encode(p, want)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := decode(p, encoded, binary.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(want, decoded) {
		t.Fatalf("got %#v", decoded)
	}

	if _, err := decode(p, encoded[:len(encoded)-1], binary.LittleEndian); err == nil {
		t.Fatal("truncated structure accepted")
	}
}

func TestStringWire(t *testing.T) {
	p := config.Parameter{Name: "TEXT", Type: "STRING"}
	for _, text := range []string{"", "Lawang", "世界😀<&"} {
		data, err := encode(p, text)
		if err != nil {
			t.Fatal(err)
		}

		if string(data) != text+"\x00" {
			t.Fatalf("STRING wire = %x", data)
		}

		got, err := decode(p, data, binary.LittleEndian)
		if err != nil || got != text {
			t.Fatalf("got %q, %v", got, err)
		}
	}
}

func TestInitialFields(t *testing.T) {
	p := config.Parameter{Type: "STRUCTURE", Fields: []config.Field{{Name: "CODE", Type: "CHAR", Length: 3}, {Name: "COUNT", Type: "INT"}}}
	data, err := encode(p, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}

	got, err := decode(p, data, binary.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, map[string]any{"CODE": "", "COUNT": 0}) {
		t.Fatalf("initial: %#v", got)
	}
}

func TestXMLStructure(t *testing.T) {
	p := config.Parameter{Name: "IS_DATA", Type: "STRUCTURE", Fields: []config.Field{{Name: "CODE", Type: "CHAR", Length: 5}, {Name: "COUNT", Type: "INT"}}}
	got, err := decodeXML(p, []byte(`<IS_DATA><CODE>A&amp;B</CODE><COUNT>-7</COUNT></IS_DATA>`))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, map[string]any{"CODE": "A&B", "COUNT": -7}) {
		t.Fatalf("XML: %#v", got)
	}

	for _, input := range []string{
		`<IS_DATA><TYPO>A</TYPO></IS_DATA>`,
		`<IS_DATA><CODE>A</CODE><CODE>B</CODE></IS_DATA>`,
		`<IS_DATA><COUNT>2147483648</COUNT></IS_DATA>`,
		`<IS_DATA/><EXTRA/>`,
	} {
		if _, err := decodeXML(p, []byte(input)); err == nil {
			t.Fatalf("invalid XML accepted: %s", input)
		}
	}
}
