package integration

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ad3n/gonwrfc/gorfc"
)

func TestRFC(t *testing.T) {
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

	uri, err := url.Parse(fmt.Sprintf("rfc://127.0.0.1:%02d", number-3300))
	if err != nil {
		t.Fatal(err)
	}

	params := gorfc.ConnectionParameters{"user": "MOCK", "passwd": "MOCK", "client": "000"}
	for key, values := range uri.Query() {
		params[key] = values[0]
	}

	params["ashost"] = uri.Hostname()
	params["sysnr"] = uri.Port()
	conn, err := gorfc.ConnectionFromParams(params)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}

	description, err := conn.GetFunctionDescription("STFC_CONNECTION")
	if err != nil {
		t.Fatal(err)
	}

	if len(description.Parameters) != 3 {
		t.Fatalf("parameters = %d", len(description.Parameters))
	}

	for _, input := range []string{"Hello Lawang", "Halo 世界", strings.Repeat("A", 255), ""} {
		output, err := conn.Call("STFC_CONNECTION", map[string]any{"REQUTEXT": input})
		if err != nil {
			t.Fatal(err)
		}

		if output["ECHOTEXT"] != input || output["RESPTEXT"] != "Hello from RFC mock Lawang-Core" {
			t.Fatalf("unexpected echo: %#v", output)
		}
	}

	output, err := conn.Call("Z_LAWANG_TEST", map[string]any{"IV_NAME": "Lawang", "CV_COUNTER": 42})
	if err != nil {
		t.Fatal(err)
	}

	if output["EV_NAME"] != "Lawang" || output["EV_STATUS"] != "SUCCESS" || output["EV_COUNT"] != int32(3) || output["EV_RATIO"] != float64(1.25) || output["CV_COUNTER"] != int32(42) {
		t.Fatalf("unexpected custom output: %#v", output)
	}

	row := map[string]any{"CODE": "A001", "QUANTITY": int32(2), "PRICE": float64(12.5)}
	second := map[string]any{"CODE": "世界", "QUANTITY": int32(-3), "PRICE": float64(7.25)}
	data, err := conn.Call("Z_LAWANG_DATA", map[string]any{"IS_HEADER": row, "T_ITEMS": []any{row}, "T_ECHO": []any{row, second}})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(data["ES_HEADER"], row) {
		t.Fatalf("structure echo: %#v", data["ES_HEADER"])
	}

	if !reflect.DeepEqual(data["T_ECHO"], []any{row, second}) {
		t.Fatalf("table echo: %#v", data["T_ECHO"])
	}

	items, ok := data["T_ITEMS"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("table fixture: %#v", data["T_ITEMS"])
	}

	for _, count := range []int{0, 128} {
		rows := make([]any, count)
		for i := range rows {
			rows[i] = map[string]any{"CODE": fmt.Sprintf("%05d", i), "QUANTITY": int32(i), "PRICE": float64(i) / 2}
		}

		output, err := conn.Call("Z_LAWANG_DATA", map[string]any{"IS_HEADER": second, "T_ECHO": rows})
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(output["T_ECHO"], rows) {
			t.Fatalf("%d-row table echo differs", count)
		}
	}

	if _, err := conn.Call("Z_UNKNOWN_FUNCTION", nil); err == nil {
		t.Fatal("unknown function should fail")
	}
}

func TestRFCParallel(t *testing.T) {
	if os.Getenv("RFC_MOCK_INTEGRATION") != "1" {
		t.Skip("requires running mock")
	}

	port := os.Getenv("RFC_MOCK_PORT")
	if port == "" {
		port = "3313"
	}

	number, err := strconv.Atoi(port)
	if err != nil || number < 3300 || number > 3399 {
		t.Fatal("invalid RFC_MOCK_PORT")
	}

	for i := range 4 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			conn, err := gorfc.ConnectionFromParams(gorfc.ConnectionParameters{"ashost": "127.0.0.1", "sysnr": fmt.Sprintf("%02d", number-3300), "user": "MOCK", "passwd": "MOCK", "client": "000"})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			input := fmt.Sprintf("parallel-%d", i)
			output, err := conn.Call("STFC_CONNECTION", map[string]any{"REQUTEXT": input})
			if err != nil {
				t.Fatal(err)
			}

			if output["ECHOTEXT"] != input {
				t.Fatalf("unexpected output %#v", output)
			}
		})
	}
}
