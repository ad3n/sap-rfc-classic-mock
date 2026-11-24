package classic

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	wire "github.com/ad3n/open-rfc-go/mockbridge"
	"github.com/ad3n/sap-rfc-classic-mock/internal/config"
)

const (
	functionInterfaceRowWidth = 402
	resumableExceptionsTable  = "RESUMABLE_EXCEPTIONS"
)

func typeName(p config.Parameter) string {
	data, _ := json.Marshal(p.Fields)
	return fmt.Sprintf("ZMOCK_%X", sha256.Sum256(data))[:30]
}

func typeCode(p config.Parameter) string {
	return map[string]string{"CHAR": "C", "STRING": "g", "INT": "I", "FLOAT": "F", "STRUCTURE": "u", "TABLE": "u"}[p.Type]
}

func tables(req wire.Request, name string, width int, rows [][]byte) []wire.Table {
	answer := wire.Table{Name: name, RowByteLength: width, Rows: rows}
	for _, table := range req.Tables {
		if table.Name == name {
			answer.ID = table.ID
		}
	}

	return []wire.Table{answer}
}

func interfaceHandler(functions map[string]config.Function) wire.Handler {
	return func(_ context.Context, req wire.Request) (wire.Response, error) {
		name, _ := req.ImportText("FUNCNAME")
		fn, ok := functions[name]
		if !ok {
			return wire.Response{}, &wire.Exception{Key: "FU_NOT_FOUND"}
		}

		flag, _ := req.ImportText("NONE_UNICODE_LENGTH")
		var rows [][]byte
		for i, p := range fn.Parameters {
			direction := map[string]string{"import": "I", "export": "E", "changing": "C", "tables": "T"}[p.Direction]
			row := chars(direction, 1)
			row = append(row, chars(p.Name, 30)...)
			tabname := ""
			if len(p.Fields) > 0 {
				tabname = typeName(p)
			}

			row = append(row, chars(tabname, 30)...)
			row = append(row, chars("", 30)...)
			row = append(row, chars(typeCode(p), 1)...)
			width, _ := dimensions(p, flag != "X")
			for _, n := range []int{i + 1, 0, width, 0} {
				row = append(row, integer(n)...)
			}

			row = append(row, chars("", 21)...)
			row = append(row, chars("", 79)...)
			row = append(row, chars("", 1)...)
			rows = append(rows, row)
		}

		responseTables := tables(req, "PARAMS", functionInterfaceRowWidth, rows)
		for _, table := range req.Tables {
			if table.Name != resumableExceptionsTable {
				continue
			}

			table.Rows = nil
			if table.RowByteLength == 0 {
				table.RowByteLength = functionInterfaceRowWidth
			}

			responseTables = append(responseTables, table)
			break
		}

		if len(responseTables) == 1 && slices.Contains(req.RequestedOutputs, resumableExceptionsTable) {
			responseTables = append(responseTables, wire.Table{Name: resumableExceptionsTable, RowByteLength: functionInterfaceRowWidth})
		}

		return wire.Response{
			Exports: []wire.NamedValue{{Name: "REMOTE_CALL", Value: chars("R", 1)}, {Name: "REMOTE_BASXML_SUPPORTED", Value: chars("", 1)}, {Name: "UPDATE_TASK", Value: chars("", 1)}},
			Tables:  responseTables,
		}, nil
	}
}

func structureHandler(types map[string]config.Parameter) wire.Handler {
	return func(_ context.Context, req wire.Request) (wire.Response, error) {
		name, _ := req.ImportText("TABNAME")
		p, ok := types[name]
		if !ok {
			return wire.Response{}, &wire.Exception{Key: "NOT_FOUND"}
		}

		flag, _ := req.ImportText("NONE_UNICODE_LENGTH")
		l := geometry(p.Fields, flag != "X")
		var rows [][]byte
		for i, f := range p.Fields {
			row := chars(name, 30)
			row = append(row, chars(f.Name, 30)...)
			width, _ := dimensions(f.Parameter(), flag != "X")
			for _, n := range []int{i + 1, l.offsets[i], width, 0} {
				row = append(row, integer(n)...)
			}

			row = append(row, chars(typeCode(f.Parameter()), 1)...)
			rows = append(rows, row)
		}

		return wire.Response{Exports: []wire.NamedValue{{Name: "TABLENGTH", Value: integer(l.size)}}, Tables: tables(req, "FIELDS", 138, rows)}, nil
	}
}

func dictionaryHandler(types map[string]config.Parameter) wire.Handler {
	return func(_ context.Context, req wire.Request) (wire.Response, error) {
		name, _ := req.ImportText("TABNAME")
		p, ok := types[name]
		if !ok {
			return wire.Response{}, &wire.Exception{Key: "NOT_FOUND"}
		}

		uclen := req.ImportByte("UCLEN")
		if uclen == 0 {
			uclen = 2
		}

		l := geometry(p.Fields, uclen != 1)
		rows := make([][]byte, 0, len(p.Fields))
		for i, f := range p.Fields {
			width, _ := dimensions(f.Parameter(), uclen != 1)
			display := f.Length
			if f.Type == "INT" {
				display = 10
			}

			if f.Type == "FLOAT" {
				display = 16
			}

			row, err := wire.DFIESRow(map[string]string{
				"TABNAME": name, "FIELDNAME": f.Name, "LANGU": "E", "POSITION": strconv.Itoa(i + 1),
				"OFFSET": strconv.Itoa(l.offsets[i]), "LENG": strconv.Itoa(display), "INTLEN": strconv.Itoa(width),
				"OUTPUTLEN": strconv.Itoa(display), "DATATYPE": map[string]string{"CHAR": "CHAR", "INT": "INT4", "FLOAT": "FLTP"}[f.Type],
				"INTTYPE": typeCode(f.Parameter()), "PRECFIELD": name, "COMPTYPE": "E", "LFIELDNAME": f.Name,
			})
			if err != nil {
				return wire.Response{}, err
			}

			rows = append(rows, row)
		}

		response := wire.Response{Exports: []wire.NamedValue{{Name: "DDOBJTYPE", Value: chars("INTTAB", 8)}}, Tables: tables(req, "DFIES_TAB", 1350, rows)}
		for _, output := range req.RequestedOutputs {
			switch output {
			case "DFIES_WA":
				row, err := wire.DFIESRow(nil)
				if err != nil {
					return wire.Response{}, err
				}

				response.Exports = append(response.Exports, wire.NamedValue{Name: output, Value: row})
			case "X030L_WA":
				header, err := wire.TypeHeader(name, l.size, len(p.Fields), uclen)
				if err != nil {
					return wire.Response{}, err
				}

				response.Exports = append(response.Exports, wire.NamedValue{Name: output, Value: header})
			case "LINES_DESCR":
				response.XrfcParameters = append(response.XrfcParameters, wire.NamedValue{Name: output, Value: []byte("<LINES_DESCR/>")})
			}
		}

		return response, nil
	}
}
