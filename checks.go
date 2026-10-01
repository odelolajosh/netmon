package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

// CheckResult is the outcome of one check at one network layer.
type CheckResult struct {
	Layer   string // "DNS", "TCP", "HTTP", ...
	OK      bool
	Latency time.Duration // how long the check took
	Detail  string        // extra info on success (e.g. resolved IP)
	Err     error         // why it failed, if it failed
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

// TODO(day 1): checkHTTP
//   - Build a URL from host (https:// if port is 443, else http://).
//   - Use an http.Client with Timeout set.
//   - Record status code and total latency.
//   - For HTTPS, read resp.TLS.PeerCertificates[0].NotAfter and
//     report how many days until the certificate expires.
//   - Treat status >= 500 as a failure.
func checkHTTP(host string, port int, timeout time.Duration) CheckResult {
	return CheckResult{Layer: "HTTP", Err: fmt.Errorf("not implemented yet")}
}

// TODO(day 1): checkPing
//   - Run the system ping via exec.Command("ping", "-c", "4", host).
//   - Parse the summary lines for packet loss % and avg rtt.
//   - Many hosts block ICMP, so a ping failure should be reported
//     but should NOT count as the target being down.
