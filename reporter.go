package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// reporter is the only goroutine that handles results, so the
// SQLite writes and alert state live here without needing a mutex.
func reporter(store *Store, alerter *Alerter, in <-chan Report) {
	printHeader := true
	const row = "%-8s  %-14s  %-6s  %-6s  %-6s  %s\n"

	for r := range in {
		if err := store.SaveReport(r); err != nil {
			log.Printf("save report: %v", err)
		}

		if alert, ok := alerter.Observe(r); ok {
			printAlert(alert)
		}

		status := "UP"
		diagnosis := "all layers OK"
		if r.FailedLayer != "" {
			status = "DOWN"
			last := r.Results[len(r.Results)-1]
			diagnosis = fmt.Sprintf("failed at %s: %v", r.FailedLayer, last.Err)
		}

		if printHeader {
			fmt.Printf(row, "TIME", "TARGET", "STATUS", "DNS", "TCP", "DIAGNOSIS")
		}

		fmt.Printf(row,
			time.Now().Format("15:04:05"), r.Target.Name, status,
			latencyFor(r, "DNS"), latencyFor(r, "TCP"), diagnosis)

		printHeader = false
	}
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

const (
	colorReset = "\033[0m"
	colorRed   = "\033[1;31m" // bold red
	colorGreen = "\033[1;32m" // bold green
)

var useColor = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

func printAlert(a Alert) {
	ts := a.At.Format("15:04:05")

	var color, msg string
	switch a.Status {
	case "DOWN":
		color = colorRed
		msg = fmt.Sprintf("▼ DOWN       %s  failed at %s: %s", a.Target, a.Layer, a.Err)
	case "RECOVERED":
		color = colorGreen
		if a.Downtime > 0 {
			msg = fmt.Sprintf("▲ RECOVERED  %s  after %s", a.Target, a.Downtime.Round(time.Second))
		} else {
			msg = fmt.Sprintf("▲ RECOVERED  %s", a.Target)
		}
	default:
		msg = fmt.Sprintf("%s  %s", a.Status, a.Target)
	}

	if useColor && color != "" {
		fmt.Printf("%s  %s%s%s\n", ts, color, msg, colorReset)
	} else {
		fmt.Printf("%s  ALERT %s\n", ts, msg)
	}
}
