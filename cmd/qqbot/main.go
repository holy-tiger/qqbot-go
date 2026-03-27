package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/openclaw/qqbot/internal/qqbot"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	// Handle subcommands: "qqbot channel ..."
	if len(os.Args) > 1 && os.Args[1] == "channel" {
		runChannel(os.Args[2:])
		return
	}

	cfgPath := flag.String("config", "", "path to YAML config file")
	healthAddr := flag.String("health", ":8080", "health check HTTP address (empty to disable)")
	apiAddr := flag.String("api", ":9090", "HTTP API server address (empty to disable)")
	flag.Parse()

	if *cfgPath == "" {
		if _, err := os.Stat("configs/config.yaml"); err == nil {
			*cfgPath = "configs/config.yaml"
		} else {
			log.Fatal("usage: qqbot -config <path> [-health <addr>] [-api <addr>]")
		}
	}

	if err := qqbot.Run(context.Background(), *cfgPath, *healthAddr, *apiAddr, version); err != nil {
		log.Fatalf("qqbot: %v", err)
	}
}

func runChannel(args []string) {
	fs := flag.NewFlagSet("channel", flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to YAML config file")
	fs.Parse(args)

	if *cfgPath == "" {
		if _, err := os.Stat("configs/config.yaml"); err == nil {
			*cfgPath = "configs/config.yaml"
		} else {
			log.Fatal("usage: qqbot channel -config <path>")
		}
	}

	if err := qqbot.RunChannel(*cfgPath); err != nil {
		log.Fatalf("qqbot channel: %v", err)
	}
}
