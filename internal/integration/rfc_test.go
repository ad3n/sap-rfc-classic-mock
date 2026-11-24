package integration

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ad3n/open-rfc-go/rfc"
)

func openMock(t *testing.T) *rfc.Client {
	t.Helper()
	if os.Getenv("RFC_MOCK_INTEGRATION") != "1" {
		t.Skip("start the mock server and set RFC_MOCK_INTEGRATION=1")
	}

	port := os.Getenv("RFC_MOCK_PORT")
	if port == "" {
		port = "3313"
	}

	number, err := strconv.Atoi(port)
	if err != nil || number < 3300 || number > 3399 {
		t.Fatal("RFC_MOCK_PORT must be 3300..3399 for host/sysnr test")
	}

	client, err := rfc.Open(t.Context(), rfc.Destination{
		Host: "127.0.0.1", Port: number, Service: fmt.Sprintf("sapdp%02d", number-3300),
		User: "MOCK", Password: "MOCK", Client: "000",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := client.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})

	return client
}

func TestRFC(t *testing.T) {
	conn := openMock(t)
	ctx := t.Context()
	description, err := conn.DescribeTool(ctx, "STFC_CONNECTION")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := description.InputSchema["properties"].(map[string]any)["REQUTEXT"]; !ok {
		t.Fatalf("missing REQUTEXT: %#v", description.InputSchema)
	}
	if outputs := description.OutputSchema["properties"].(map[string]any); len(outputs) != 2 {
		t.Fatalf("unexpected outputs: %#v", outputs)
	}

	for _, input := range []string{"Hello Lawang", "Halo 世界", strings.Repeat("A", 255), ""} {
		output, err := conn.Call(ctx, "STFC_CONNECTION", map[string]any{"REQUTEXT": input})
		if err != nil {
			t.Fatal(err)
		}

		if output.Get("ECHOTEXT") != input || output.Get("RESPTEXT") != "Hello from RFC mock Lawang-Core" {
			t.Fatalf("unexpected echo: %#v", output)
		}
	}

	output, err := conn.Call(ctx, "Z_LAWANG_TEST", map[string]any{"IV_NAME": "Lawang", "CV_COUNTER": 42})
	if err != nil {
		t.Fatal(err)
	}

	if output.Get("EV_NAME") != "Lawang" || output.Get("EV_STATUS") != "SUCCESS" || output.Get("EV_COUNT") != int32(3) || output.Get("EV_RATIO") != float64(1.25) || output.Get("CV_COUNTER") != int32(42) {
		t.Fatalf("unexpected custom output: %#v", output)
	}

	row := map[string]any{"CODE": "A001", "QUANTITY": int32(2), "PRICE": float64(12.5)}
	second := map[string]any{"CODE": "世界", "QUANTITY": int32(-3), "PRICE": float64(7.25)}
	data, err := conn.Call(ctx, "Z_LAWANG_DATA", map[string]any{"IS_HEADER": row, "T_ITEMS": []any{row}, "T_ECHO": []map[string]any{row, second}})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(data.Get("ES_HEADER"), row) {
		t.Fatalf("structure echo: %#v", data.Get("ES_HEADER"))
	}

	if !reflect.DeepEqual(data.Table("T_ECHO"), []map[string]any{row, second}) {
		t.Fatalf("table echo: %#v", data.Table("T_ECHO"))
	}

	if items := data.Table("T_ITEMS"); len(items) != 2 {
		t.Fatalf("table fixture: %#v", items)
	}

	for _, count := range []int{0, 128} {
		rows := make([]map[string]any, count)
		for i := range rows {
			rows[i] = map[string]any{"CODE": fmt.Sprintf("%05d", i), "QUANTITY": int32(i), "PRICE": float64(i) / 2}
		}

		output, err := conn.Call(ctx, "Z_LAWANG_DATA", map[string]any{"IS_HEADER": second, "T_ECHO": rows})
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(output.Table("T_ECHO"), rows) {
			t.Fatalf("%d-row table echo differs", count)
		}
	}

	if _, err := conn.Call(ctx, "Z_UNKNOWN_FUNCTION", nil); err == nil {
		t.Fatal("unknown function should fail")
	}
}

func TestRFCParallel(t *testing.T) {
	for i := range 4 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			conn := openMock(t)

			input := fmt.Sprintf("parallel-%d", i)
			output, err := conn.Call(t.Context(), "STFC_CONNECTION", map[string]any{"REQUTEXT": input})
			if err != nil {
				t.Fatal(err)
			}

			if output.Get("ECHOTEXT") != input {
				t.Fatalf("unexpected output %#v", output)
			}
		})
	}
}
