package main

import "time"

// targetState is everything the alerter remembers about one target.
type targetState struct {
	confirmed string    // last status we alerted on: "UP" or "DOWN"
	failures  int       // consecutive DOWN probes
	downSince time.Time // zero if unknown (e.g. was DOWN before a restart)
}

// Alerter decides when a target's status has really changed.
// It is not safe for concurrent use: only the reporter goroutine owns it.
type Alerter struct {
	threshold int
	targets   map[string]*targetState
}

type Alert struct {
	Target   string
	Status   string
	Layer    string
	At       time.Time
	Downtime time.Duration
	Err      string
}

// NewAlerter seeds state from the statuses saved before the last shutdown.
func NewAlerter(last map[string]string, threshold int) *Alerter {
	if threshold < 1 {
		threshold = 1
	}
	a := &Alerter{threshold: threshold, targets: make(map[string]*targetState)}
	for name, status := range last {
		a.targets[name] = &targetState{confirmed: status}
	}
	return a
}

// Observe records one report and returns an alert if the confirmed
// status changed.
func (a *Alerter) Observe(r Report) (Alert, bool) {
	name := r.Target.Name
	down := r.FailedLayer != ""

	st, seen := a.targets[name]
	if !seen {
		// New target: remember it, but don't alert on first sight.
		st = &targetState{confirmed: "UP"}
		if down {
			st.confirmed = "DOWN"
			st.downSince = r.At
		}
		a.targets[name] = st
		return Alert{}, false
	}

	if !down {
		st.failures = 0
		if st.confirmed != "DOWN" {
			return Alert{}, false
		}
		alert := Alert{Target: name, Status: "RECOVERED", At: r.At}
		if !st.downSince.IsZero() {
			alert.Downtime = r.At.Sub(st.downSince)
		}
		st.confirmed, st.downSince = "UP", time.Time{}
		return alert, true
	}

	st.failures++
	if st.confirmed == "DOWN" || st.failures < a.threshold {
		return Alert{}, false
	}
	last := r.Results[len(r.Results)-1]
	alert := Alert{Target: name, Status: "DOWN", Layer: r.FailedLayer, At: r.At}
	if last.Err != nil {
		alert.Err = last.Err.Error()
	}
	st.confirmed, st.downSince = "DOWN", r.At
	return alert, true
}
