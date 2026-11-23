package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ad3n/sap-rfc-classic-mock/internal/classic"
	"github.com/ad3n/sap-rfc-classic-mock/internal/config"
	"github.com/ad3n/sap-rfc-classic-mock/internal/server"
)

func main() {
	path := flag.String("config", "mock.yml", "YAML configuration")
	listenHost := flag.String("listen-host", "", "override classic RFC listen host")
	validate := flag.Bool("check", false, "validate configuration and exit")
	flag.Parse()
	cfg, err := config.Load(*path)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	if *listenHost != "" {
		cfg.ListenHost = *listenHost
	}

	if *validate {
		log.Print("configuration valid")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run := classic.Run
	if cfg.Transport == "websocket" {
		run = server.Run
	}

	if err := run(ctx, cfg); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
