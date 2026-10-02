package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	createStmt := `
	CREATE TABLE IF NOT EXISTS probes (
    id             INTEGER PRIMARY KEY,
    ts             INTEGER NOT NULL,          -- Unix time in milliseconds
    target         TEXT    NOT NULL,          -- target name from targets.json
    status         TEXT    NOT NULL CHECK (status IN ('UP', 'DOWN')),
    failed_layer   TEXT,                      -- 'DNS', 'TCP', 'HTTP', or NULL if UP
    dns_ms         REAL,                      -- NULL if the layer didn't run or failed
    tcp_ms         REAL,
    http_ms        REAL,
    http_status    INTEGER,
    cert_days_left INTEGER,                   -- NULL for plain HTTP
    ping_loss      REAL,                      -- percent; NULL if ping couldn't run
    ping_rtt_ms    REAL,
    error          TEXT                       -- the failed layer's error message
	);

	CREATE INDEX IF NOT EXISTS idx_probes_target_ts ON probes (target, ts);
	`
	_, err = db.Exec(createStmt)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to setup report tables: %w", err)
	}

	s := Store{}
	s.db = db
	return &s, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return err
		}
		s.db = nil
	}
	return nil
}

func (s *Store) SaveReport(r Report) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status := "UP"
	var failedLayer, errMsg any
	if r.FailedLayer != "" {
		status = "DOWN"
		failedLayer = r.FailedLayer
		if last := r.Results[len(r.Results)-1]; last.Err != nil {
			errMsg = last.Err.Error()
		}
	}

	var httpStatus, certDays any
	if h := findResult(r, "HTTP"); h != nil {
		if h.HTTPStatus != 0 {
			httpStatus = h.HTTPStatus
		}
		if h.OK && r.Target.Port == 443 {
			certDays = h.CertDaysLeft
		}
	}

	var pingLoss, pingRTT any
	if r.Ping.Detail != "" {
		pingLoss = r.Ping.PingLoss
		if r.Ping.OK {
			pingRTT = toMs(r.Ping.Latency)
		}
	}

	const insertStmt = `
		INSERT INTO probes (ts, target, status, failed_layer, dns_ms, tcp_ms, http_ms,
		                    http_status, cert_days_left, ping_loss, ping_rtt_ms, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, insertStmt,
		r.At.UnixMilli(), r.Target.Name, status, failedLayer,
		layerMs(r, "DNS"), layerMs(r, "TCP"), layerMs(r, "HTTP"),
		httpStatus, certDays, pingLoss, pingRTT, errMsg,
	)
	if err != nil {
		return fmt.Errorf("insert probe for %s: %w", r.Target.Name, err)
	}
	return nil
}

func findResult(r Report, layer string) *CheckResult {
	for i := range r.Results {
		if r.Results[i].Layer == layer {
			return &r.Results[i]
		}
	}
	return nil
}

// layerMs returns a layer's latency in ms if it ran and passed, else nil (NULL).
func layerMs(r Report, layer string) any {
	if res := findResult(r, layer); res != nil && res.OK {
		return toMs(res.Latency)
	}
	return nil
}

func toMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func (s *Store) LastStatuses() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const queryStmt = `
	SELECT p.target, p.status
	FROM probes p
	JOIN (
		SELECT target, MAX(ts) AS ts
		FROM probes
		GROUP BY target
	) latest ON p.target = latest.target AND p.ts = latest.ts`

	rows, err := s.db.QueryContext(ctx, queryStmt)
	if err != nil {
		return nil, fmt.Errorf("query latest probes: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var target, status string
		if err := rows.Scan(&target, &status); err != nil {
			return nil, fmt.Errorf("scan latest probe: %w", err)
		}
		result[target] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest probes: %w", err)
	}

	return result, nil
}

// Read: `netmon report`
// func (s *Store) Summaries(since time.Time) ([]TargetSummary, error)

type TargetSummary struct {
	Target       string
	Probes       int
	UpPercent    float64
	P95          time.Duration // over UP probes only
	LastFailure  *Failure      // nil if no failure in the window
	CertDaysLeft *int          // nil for plain-HTTP targets
}

type Failure struct {
	At    time.Time
	Layer string
	Err   string
}
