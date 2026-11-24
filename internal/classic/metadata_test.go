package classic

import (
	"context"
	"testing"

	wire "github.com/ad3n/open-rfc-go/mockbridge"
	"github.com/ad3n/sap-rfc-classic-mock/internal/config"
)

func TestFunctionInterfaceResumableExceptions(t *testing.T) {
	handler := interfaceHandler(map[string]config.Function{"Z_TEST": {Name: "Z_TEST"}})
	for _, test := range []struct {
		name      string
		request   wire.Request
		wantWidth int
	}{
		{
			name:      "requested output",
			request:   wire.Request{RequestedOutputs: []string{resumableExceptionsTable}},
			wantWidth: functionInterfaceRowWidth,
		},
		{
			name:      "table with explicit geometry",
			request:   wire.Request{Tables: []wire.Table{{Name: resumableExceptionsTable, RowByteLength: 60}}},
			wantWidth: 60,
		},
		{
			name:      "table without geometry",
			request:   wire.Request{Tables: []wire.Table{{Name: resumableExceptionsTable}}},
			wantWidth: functionInterfaceRowWidth,
		},
		{name: "unrequested"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.request.Imports = []wire.NamedValue{{Name: "FUNCNAME", Value: chars("Z_TEST", 30)}}
			response, err := handler(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}

			if test.wantWidth == 0 {
				if len(response.Tables) != 1 {
					t.Fatalf("unexpected tables: %#v", response.Tables)
				}

				return
			}

			if len(response.Tables) != 2 {
				t.Fatalf("expected PARAMS and %s, got %#v", resumableExceptionsTable, response.Tables)
			}

			table := response.Tables[1]
			if table.Name != resumableExceptionsTable || table.RowByteLength != test.wantWidth || len(table.Rows) != 0 {
				t.Fatalf("unexpected exceptions table: %#v", table)
			}
		})
	}
}
