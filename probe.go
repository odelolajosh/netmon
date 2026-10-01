package main

import (
	"context"
	"time"
)

// Report is everything we learned about one target in one run.
type Report struct {
	Target      Target
	Results     []CheckResult
	FailedLayer string // "" if everything passed
}

// probe checks a target layer by layer: DNS -> TCP -> HTTP.
// Each layer depends on the one below it, so we stop at the first
// failure. The failed layer tells us WHERE the problem is.
func probe(ctx context.Context, t Target, timeout time.Duration) Report {
	rep := Report{Target: t}

	dns, addrs := checkDNS(ctx, t.Host, timeout)
	rep.Results = append(rep.Results, dns)
	if !dns.OK {
		rep.FailedLayer = "DNS"
		return rep
	}

	tcp := checkTCP(addrs[0], t.Port, timeout)
	rep.Results = append(rep.Results, tcp)
	if !tcp.OK {
		rep.FailedLayer = "TCP"
		return rep
	}

	// TODO(day 1): once checkHTTP is written, uncomment:
	// http := checkHTTP(t.Host, t.Port, timeout)
	// rep.Results = append(rep.Results, http)
	// if !http.OK {
	// 	rep.FailedLayer = "HTTP"
	// 	return rep
	// }

	return rep
}
