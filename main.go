package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"
)

func main() {
	configPath := flag.String("config", "targets.json", "path to targets file")
	dbPath := flag.String("db", "netmon.db", "path to SQLite database")
	flag.Parse()

	if err := run(*configPath, *dbPath); err != nil {
		log.Fatal(err)
	}
}

func run(configPath string, dbPath string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	store, err := NewStore(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	statuses, err := store.LastStatuses()
	if err != nil {
		return err
	}

	alerter := NewAlerter(statuses, cfg.FailThreshold)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	reports := make(chan Report)
	done := make(chan struct{})

	go func() {
		reporter(store, alerter, reports)
		close(done) // signals that the reporter has finished
	}()

	ticker := time.NewTicker(cfg.Timeout())
	defer ticker.Stop()

	for {
		probeAll(ctx, cfg, reports)

		select {
		case <-ctx.Done():
			close(reports)
			<-done // wait for it to finish writing
			return nil
		case <-ticker.C:
		}

		<-ticker.C
	}
}

func probeAll(ctx context.Context, cfg Config, out chan<- Report) {
	var wg sync.WaitGroup

	for i, t := range cfg.Targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			out <- probe(ctx, t, cfg.Timeout())
		}(i, t)
	}

	wg.Wait()
}
