package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ad3n/open-rfc-go/rfc"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	host := flag.String("host", "127.0.0.1", "RFC host")
	port := flag.Int("port", 3313, "RFC port (3300..3399)")
	transport := flag.String("transport", "classic", "RFC transport (classic)")
	describe := flag.Bool("describe", false, "print function metadata")
	function := flag.String("function", "STFC_CONNECTION", "RFC function")
	input := flag.String("params", `{"REQUTEXT":"Hello Lawang"}`, "JSON input parameters")
	flag.Parse()
	params, err := parseInput(*input)
	if err != nil {
		return err
	}

	if *transport != "classic" {
		return fmt.Errorf("unknown transport %q", *transport)
	}

	if *port < 3300 || *port > 3399 {
		return fmt.Errorf("classic port must be 3300..3399 (sysnr 00..99)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := rfc.Open(ctx, rfc.Destination{
		Host: *host, Port: *port, Service: fmt.Sprintf("sapdp%02d", *port-3300),
		Client: "000", User: "MOCK", Password: "MOCK", Language: "E",
	})
	if err != nil {
		return err
	}
	defer client.Close(context.WithoutCancel(ctx))

	description, err := client.DescribeTool(ctx, *function)
	if err != nil {
		return err
	}

	var result any = description
	if !*describe {
		if err := validateInput(params, description.InputSchema, ""); err != nil {
			return err
		}

		result, err = client.Call(ctx, *function, params)
		if err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}
