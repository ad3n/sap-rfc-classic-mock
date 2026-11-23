package classic

import (
	"context"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	wire "github.com/ad3n/open-rfc-go/mockbridge"
	"github.com/ad3n/sap-rfc-classic-mock/internal/config"
)

func Run(ctx context.Context, cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	host := cfg.ListenHost
	if host == "" {
		host = "127.0.0.1"
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(cfg.Port)))
	if err != nil {
		return err
	}
	defer listener.Close()

	dispatcher := wire.NewDispatcher()
	functions := make(map[string]config.Function)
	types := make(map[string]config.Parameter)
	for _, fn := range cfg.Functions {
		functions[fn.Name] = fn
		dispatcher.Handle(fn.Name, functionHandler(fn))
		for _, p := range fn.Parameters {
			if len(p.Fields) > 0 {
				types[typeName(p)] = p
			}
		}
	}

	dispatcher.Handle("RFC_PING", func(context.Context, wire.Request) (wire.Response, error) { return wire.Response{}, nil })
	dispatcher.Handle("RFC_GET_FUNCTION_INTERFACE", interfaceHandler(functions))
	dispatcher.Handle("RFC_GET_STRUCTURE_DEFINITION", structureHandler(types))
	dispatcher.Handle("DDIF_FIELDINFO_GET", dictionaryHandler(types))
	log.Printf("Classic RFC mock listening on %s; %d functions; local test logon accepts any credentials", listener.Addr(), len(functions))
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer workers.Wait()
	defer cancel()

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			listener.Close()
		case <-done:
		}
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return err
		}

		workers.Add(1)
		go func() {
			defer workers.Done()

			closed := make(chan struct{})
			defer close(closed)

			go func() {
				select {
				case <-ctx.Done():
					conn.Close()
				case <-closed:
				}
			}()
			conn.SetDeadline(time.Now().Add(5 * time.Minute))
			wire.Serve(conn, dispatcher, func(message string) { log.Printf("RFC %s: %s", conn.RemoteAddr(), message) })
		}()
	}
}

func functionHandler(fn config.Function) wire.Handler {
	return func(_ context.Context, req wire.Request) (wire.Response, error) {
		values := make(map[string]any)
		for _, p := range fn.Parameters {
			if p.Direction == "export" {
				continue
			}

			value := initial(p)
			for _, input := range req.Imports {
				if input.Name != p.Name {
					continue
				}

				decoded, err := decode(p, input.Value, req.Order)
				if err != nil {
					return wire.Response{}, err
				}

				value = decoded
			}

			if p.Type == "TABLE" {
				rows := []any{}
				for _, table := range req.Tables {
					if table.Name != p.Name {
						continue
					}

					for _, data := range table.Rows {
						row, err := decode(config.Parameter{Name: p.Name, Type: "STRUCTURE", Fields: p.Fields}, data, req.Order)
						if err != nil {
							return wire.Response{}, err
						}

						rows = append(rows, row)
					}
				}

				value = rows
			}

			for _, input := range req.XrfcParameters {
				if input.Name != p.Name {
					continue
				}

				decoded, err := decodeXML(p, input.Value)
				if err != nil {
					return wire.Response{}, err
				}

				value = decoded
			}

			values[p.Name] = value
		}

		var response wire.Response
		for _, p := range fn.Parameters {
			if p.Direction == "import" {
				continue
			}

			value := p.Value
			if p.From != "" {
				value = values[p.From]
			}

			if value == nil && (p.Direction == "changing" || p.Direction == "tables") {
				value = values[p.Name]
			}

			if value == nil {
				value = initial(p)
			}

			if p.Type == "TABLE" {
				rows := make([][]byte, 0)
				for _, value := range value.([]any) {
					row, err := encode(config.Parameter{Type: "STRUCTURE", Fields: p.Fields}, value)
					if err != nil {
						return wire.Response{}, err
					}

					rows = append(rows, row)
				}

				response.Tables = append(response.Tables, tables(req, p.Name, geometry(p.Fields, true).size, rows)...)
				continue
			}

			data, err := encode(p, value)
			if err != nil {
				return wire.Response{}, err
			}

			response.Exports = append(response.Exports, wire.NamedValue{Name: p.Name, Value: data})
		}

		return response, nil
	}
}
