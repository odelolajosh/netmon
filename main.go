package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"text/tabwriter"
	"time"
)

func main() {
	configPath := flag.String("config", "targets.json", "path to targets file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	reports := runOnce(context.Background(), cfg)
	printReports(reports)

	// TODO(day 2): instead of running once, loop on a time.Ticker,
	// save each Report to SQLite, and alert when a target changes
	// from UP to DOWN (or back).
}

// runOnce probes every target concurrently, one goroutine per target.
func runOnce(ctx context.Context, cfg Config) []Report {
	reports := make([]Report, len(cfg.Targets))
	var wg sync.WaitGroup

	for i, t := range cfg.Targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			// Each goroutine writes only to its own index, so no mutex is needed.
			reports[i] = probe(ctx, t, cfg.Timeout())
		}(i, t)
	}

	wg.Wait()
	return reports
}

func printReports(reports []Report) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TARGET\tSTATUS\tDNS\tTCP\tDIAGNOSIS")

	for _, r := range reports {
		status := "UP"
		diagnosis := "all layers OK"
		if r.FailedLayer != "" {
			status = "DOWN"
			last := r.Results[len(r.Results)-1]
			diagnosis = fmt.Sprintf("failed at %s: %v", r.FailedLayer, last.Err)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.Target.Name, status,
			latencyFor(r, "DNS"), latencyFor(r, "TCP"), diagnosis)
	}
	w.Flush()
}

// latencyFor returns the latency of a layer, or "-" if it never ran or failed.
func latencyFor(r Report, layer string) string {
	for _, res := range r.Results {
		if res.Layer == layer && res.OK {
			return res.Latency.Round(time.Millisecond).String()
		}
	}
	return "-"
}
