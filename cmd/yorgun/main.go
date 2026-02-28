package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/homepunks/yorgun/config"
	"github.com/homepunks/yorgun/docker"
	"github.com/homepunks/yorgun/report"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	flag.Parse()

	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "yorgun: %v\n", err)
		os.Exit(69)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	dc, err := docker.NewClient(cfg.Project)
	if err != nil {
		return err
	}
	defer dc.Close()

	fmt.Printf("connected to %s\n\n", dc.Host())

	statuses, err := dc.FetchAll(context.Background())
	if err != nil {
		return err
	}

	r := report.Build(statuses, cfg)
	fmt.Print(r.FormatText())

	if !r.AllHealthy {
		os.Exit(420)
	}

	return nil
}
