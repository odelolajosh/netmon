package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// CheckResult is the outcome of one check at one network layer.
type CheckResult struct {
	Layer        string // "DNS", "TCP", "HTTP", ...
	OK           bool
	Latency      time.Duration // how long the check took
	Detail       string        // extra info on success (e.g. resolved IP)
	Err          error         // why it failed, if it failed
	HTTPStatus   int
	CertDaysLeft int
	PingLoss     float64
}

// checkDNS resolves the host to IP addresses.
// Latency here is the DNS lookup time.
func checkDNS(ctx context.Context, host string, timeout time.Duration) (CheckResult, []string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	res := CheckResult{Layer: "DNS", Latency: time.Since(start)}
	if err != nil {
		res.Err = err
		return res, nil
	}
	res.OK = true
	res.Detail = addrs[0]
	return res, addrs
}

// checkTCP opens a TCP connection to ip:port.
// Latency here is roughly the TCP three-way handshake time.
func checkTCP(ip string, port int, timeout time.Duration) CheckResult {
	// JoinHostPort adds [brackets] for IPv6 addresses.
	addr := net.JoinHostPort(ip, strconv.Itoa(port))

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	res := CheckResult{Layer: "TCP", Latency: time.Since(start)}
	if err != nil {
		res.Err = err
		return res
	}
	conn.Close()
	res.OK = true
	res.Detail = addr
	return res
}

func checkHTTP(host string, port int, timeout time.Duration) CheckResult {
	scheme := "http"
	if port == 443 {
		scheme = "https"
	}

	// Only add the port to the URL when it isn't the scheme's default.
	hostPort := host
	if port != 80 && port != 443 {
		hostPort = net.JoinHostPort(host, strconv.Itoa(port))
	}
	u := url.URL{Scheme: scheme, Host: hostPort, Path: "/"}

	client := &http.Client{Timeout: timeout}

	start := time.Now()
	resp, err := client.Get(u.String())
	res := CheckResult{Layer: "HTTP", Latency: time.Since(start)}
	if err != nil {
		res.Err = err
		return res
	}
	defer resp.Body.Close()

	res.HTTPStatus = resp.StatusCode
	res.Detail = resp.Status

	if resp.StatusCode >= 500 {
		res.Err = fmt.Errorf("server error: %s", resp.Status)
		return res
	}

	// resp.TLS is nil if the final response (after redirects) was plain HTTP.
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		daysLeft := int(time.Until(resp.TLS.PeerCertificates[0].NotAfter).Hours() / 24)
		res.CertDaysLeft = daysLeft
		if daysLeft < 14 {
			res.Detail += fmt.Sprintf(" (WARNING: certificate expires in %d days)", daysLeft)
		}
	}

	res.OK = true
	return res
}

var (
	lossRe = regexp.MustCompile(`([\d.]+)% packet loss`)
	rttRe  = regexp.MustCompile(`= [\d.]+/([\d.]+)/`)
)

func checkPing(ctx context.Context, host string, timeout time.Duration) CheckResult {
	res := CheckResult{Layer: "Ping"}

	// 4 pings at 1s intervals take ~3s, so allow that on top of the timeout.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second+timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ping", "-c", "4", host).CombinedOutput()

	loss, avg, perr := parsePing(string(out))
	if perr != nil {
		if err != nil {
			res.Err = fmt.Errorf("ping failed: %v", err)
		} else {
			res.Err = perr
		}
		return res
	}

	res.PingLoss = loss
	res.Latency = avg
	res.Detail = fmt.Sprintf("%.0f%% loss, avg %v", loss, avg.Round(time.Millisecond))
	if loss >= 100 {
		res.Err = fmt.Errorf("no replies (100%% packet loss)")
		return res
	}
	res.OK = true
	return res
}

// parsePing extracts packet loss % and average RTT from ping's summary.
func parsePing(out string) (float64, time.Duration, error) {
	m := lossRe.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, fmt.Errorf("no packet-loss line in ping output")
	}
	loss, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, 0, err
	}

	var avg time.Duration
	if r := rttRe.FindStringSubmatch(out); r != nil { // absent at 100% loss
		ms, err := strconv.ParseFloat(r[1], 64)
		if err != nil {
			return 0, 0, err
		}
		avg = time.Duration(ms * float64(time.Millisecond))
	}
	return loss, avg, nil
}
