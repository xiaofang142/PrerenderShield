package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"prerender-shield/internal/bootstrap"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	log.Println("Prerender Shield starting...")
	if err := bootstrap.Run(ctx, configPath); err != nil {
		log.Fatalf("Application error: %v", err)
	}

	log.Println("Prerender Shield stopped")
}
