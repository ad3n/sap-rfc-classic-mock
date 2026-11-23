package rfcserver

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/oisee/open-rfc-go/internal/cpic"
)

func TestMockNumberedInputTables(t *testing.T) {
	fields := []cpic.Field{{Tag: uint16(cpic.TagFunction), Value: encodeUTF16LE("Z_TEST")}}
	for i, name := range []string{"T_FIRST", "T_SECOND"} {
		id := make([]byte, 4)
		binary.BigEndian.PutUint32(id, uint32(i+1))
		fields = append(fields, cpic.Field{Tag: uint16(cpic.TagTableName), Value: encodeUTF16LE(name)}, cpic.Field{Tag: tagTableID, Value: id})
	}

	for _, id := range []uint32{2, 1} {
		header := make([]byte, 12)
		binary.BigEndian.PutUint32(header, 10)
		binary.BigEndian.PutUint32(header[4:], id)
		binary.BigEndian.PutUint32(header[8:], 1)
		fields = append(fields,
			cpic.Field{Tag: tagTableIDHeader, Value: header},
			cpic.Field{Tag: uint16(cpic.TagTableHeader), Value: []byte{0, 0, 0, 4, 0, 0, 0, 1}},
			cpic.Field{Tag: uint16(cpic.TagTableContent), Value: []byte{byte(id), 0, 0, 0}},
			cpic.Field{Tag: tagTableIDEnd, Value: header[4:8]},
		)
	}

	fields = append(fields, cpic.Field{Tag: uint16(cpic.TagEnd)})
	chain, err := cpic.EncodeFieldChain(uint16(cpic.TagContextEnd), fields, cpic.FieldChainLimits{})
	if err != nil {
		t.Fatal(err)
	}

	request, err := DecodeCutFunctionRequest(append(append([]byte(nil), cutRequestPrefix...), chain...))
	if err != nil {
		t.Fatal(err)
	}

	if len(request.Tables) != 2 {
		t.Fatalf("table count %d", len(request.Tables))
	}

	for i, table := range request.Tables {
		if !reflect.DeepEqual(table.Rows, [][]byte{{byte(i + 1), 0, 0, 0}}) {
			t.Fatalf("table %s has rows %x", table.Name, table.Rows)
		}
	}
}
