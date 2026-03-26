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
