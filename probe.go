package main

import (
	"context"
	"time"
)

// Report is everything we learned about one target in one run.
type Report struct {
	Target      Target
	Results     []CheckResult
	Ping        CheckResult
	FailedLayer string // "" if everything passed
	At          time.Time
}

// probe runs the layered checks, then ping. Ping always runs, even if
// an earlier layer failed, but it never affects FailedLayer.
func probe(ctx context.Context, t Target, timeout time.Duration) Report {
	rep := probeLayers(ctx, t, timeout)
	rep.Ping = checkPing(ctx, t.Host, timeout)
	return rep
}

func probeLayers(ctx context.Context, t Target, timeout time.Duration) Report {
	rep := Report{Target: t, At: time.Now()}

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

	http := checkHTTP(t.Host, t.Port, timeout)
	rep.Results = append(rep.Results, http)
	if !http.OK {
		rep.FailedLayer = "HTTP"
		return rep
	}

	return rep
}
