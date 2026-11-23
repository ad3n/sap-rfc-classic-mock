package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/ad3n/gonwrfc/gorfc"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	host := flag.String("host", "127.0.0.1", "RFC host")
	port := flag.String("port", "3313", "RFC port (classic: 3300..3399)")
	transport := flag.String("transport", "classic", "classic or websocket")
	describe := flag.Bool("describe", false, "print function metadata")
	function := flag.String("function", "STFC_CONNECTION", "RFC function")
	input := flag.String("params", `{"REQUTEXT":"Hello Lawang"}`, "JSON input parameters")
	flag.Parse()
	params, err := parseInput(*input)
	if err != nil {
		return err
	}

	connection := gorfc.ConnectionParameters{"user": "MOCK", "passwd": "MOCK", "client": "000", "lang": "EN"}
	switch *transport {
	case "classic":
		n, err := strconv.Atoi(*port)
		if err != nil || n < 3300 || n > 3399 {
			return fmt.Errorf("classic port must be 3300..3399 (sysnr 00..99)")
		}

		connection["ashost"] = *host
		connection["sysnr"] = fmt.Sprintf("%02d", n-3300)
	case "websocket":
		connection["wshost"], connection["wsport"], connection["use_tls"] = *host, *port, "0"
	default:
		return fmt.Errorf("unknown transport %q", *transport)
	}

	conn, err := gorfc.ConnectionFromParams(connection)
	if err != nil {
		return err
	}
	defer conn.Close()

	var result any
	if *describe {
		result, err = conn.GetFunctionDescription(*function)
	}

	if !*describe {
		description, describeErr := conn.GetFunctionDescription(*function)
		if describeErr != nil {
			return describeErr
		}

		if err := normalizeInput(params, description); err != nil {
			return err
		}

		result, err = conn.Call(*function, params)
	}

	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}
