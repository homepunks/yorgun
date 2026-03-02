package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/homepunks/yorgun/config"
	"github.com/homepunks/yorgun/docker"
	"github.com/homepunks/yorgun/report"

	_ "golang.org/x/crypto/x509roots/fallback"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	envPath := flag.String("env", ".env", "path to .env containing telegram bot token")
	flag.Parse()

	if err := run(*configPath, *envPath); err != nil {
		log.Fatalf("yorgun: %v", err)
	}
}

func run(configPath, envPath string) error {
	cfg, err := config.Load(configPath, envPath)
	if err != nil {
		return err
	}

	dc, err := docker.NewClient(cfg.Project)
	if err != nil {
		return err
	}
	defer dc.Close()

	log.Printf("connected to %s\n\n", dc.Host())

	statuses, err := dc.FetchAll(context.Background())
	if err != nil {
		return fmt.Errorf("initial fetch: %w", err)
	}

	startupReport := report.FormatStartupReport(statuses, cfg)
	fmt.Print(startupReport)

	bot := report.NewBot(cfg.Telegram.BotToken, cfg.Telegram.ChatIDs)
	if err := bot.Broadcast(startupReport); err != nil {
		return fmt.Errorf("telegram: %w", err)
	}

	msg := "yorgun started - watching [" + cfg.Project + "]"
	if err := bot.Broadcast(msg); err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	log.Println("telegram connected - startup message sent")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("watching for events...")

	watched := cfg.WatchedServices()
	events, errs := dc.Watch(ctx)

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil
			}

			if !watched[ev.Service] {
				continue
			}

			critical := cfg.IsCritical(ev.Service)
			msg := report.FormatEvent(ev.Service, ev.Action, ev.ExitCode, critical)

			log.Printf("[%s] %s\n", ev.Time.Format("15:04:05"), msg)

			if err := bot.Broadcast(msg); err != nil {
				log.Printf("telegram error: %v\n", err)
			}

		case err, ok := <-errs:
			if !ok {
				return nil
			}
			log.Printf("event stream error: %w\n", err)

		case <-ctx.Done():
			log.Println("\nshutting down...")

			msg := "yorgun stopped - no longer watching [" + cfg.Project + "]"
			bot.Broadcast(msg)

			return nil
		}
	}
}
