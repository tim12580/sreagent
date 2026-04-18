package datasource

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VictoriaLogsChecker checks VictoriaLogs health.
type VictoriaLogsChecker struct{}

// CheckHealth performs a two-phase probe:
//  1. GET /health — basic liveness
//  2. POST /select/logsql/query with `* | limit 0` — verifies the LogsQL engine responds
func (c *VictoriaLogsChecker) CheckHealth(ctx context.Context, endpoint, authType, authConfig string) HealthResult {
	base := strings.TrimRight(endpoint, "/")

	// ── Phase 1: liveness ────────────────────────────────────────────────────
	if err := httpGet(ctx, base+"/health", authType, authConfig); err != nil {
		return HealthResult{Healthy: false, LatencyMs: -1,
			Message: fmt.Sprintf("liveness probe failed: %v", err)}
	}

	// ── Phase 2: LogsQL engine probe ─────────────────────────────────────────
	start := time.Now()
	apiURL := base + "/select/logsql/query"
	now := time.Now()
	form := url.Values{}
	form.Set("query", "*")
	form.Set("start", now.Add(-1*time.Minute).Format(time.RFC3339))
	form.Set("end", now.Format(time.RFC3339))
	form.Set("limit", "0") // zero limit — just test API responds, no data transfer

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return HealthResult{Healthy: false, LatencyMs: time.Since(start).Milliseconds(),
			Message: fmt.Sprintf("failed to build query request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyAuth(req, authType, authConfig)

	resp, err := httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return HealthResult{Healthy: false, LatencyMs: latency,
			Message: fmt.Sprintf("query API unreachable: %v", err)}
	}
	defer resp.Body.Close()
	// VictoriaLogs returns 200 OK even with 0 results; any non-200 is a problem.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return HealthResult{Healthy: false, LatencyMs: latency,
			Message: fmt.Sprintf("query API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	return HealthResult{Healthy: true, LatencyMs: latency, Message: "VictoriaLogs is healthy"}
}

// VictoriaLogsInstantQuery executes a LogsQL query against VictoriaLogs and returns
// the count of matching log entries as a single QueryResult.
//
// The expression is a LogsQL query string (e.g. `error level:error _time:5m`).
// The result value is the number of log lines returned by the query.
// A time range of the last 5 minutes is used if not specified in the expression.
//
// VictoriaLogs API: POST /select/logsql/query
// Response format: NDJSON — one JSON object per line.
func VictoriaLogsInstantQuery(ctx context.Context, endpoint, authType, authConfig, expression string) ([]QueryResult, error) {
	apiURL := strings.TrimRight(endpoint, "/") + "/select/logsql/query"

	// Build form body with the LogsQL query and a 5-minute time window
	now := time.Now()
	form := url.Values{}
	form.Set("query", expression)
	form.Set("start", now.Add(-5*time.Minute).Format(time.RFC3339))
	form.Set("end", now.Format(time.RFC3339))
	// Limit to 10000 lines max to avoid memory issues
	form.Set("limit", "10000")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create logsql query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyAuth(req, authType, authConfig)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("logsql query request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("logsql query returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse NDJSON response: each line is a JSON log entry.
	// We count the number of lines to get the log count value.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB per line max

	var count float64
	// Collect distinct stream labels from the first few entries to enrich result labels
	streamLabels := make(map[string]string)
	firstEntry := true

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		count++

		// Parse first entry to extract stream labels (e.g. job, instance)
		if firstEntry {
			firstEntry = false
			var entry map[string]interface{}
			if err := json.Unmarshal(line, &entry); err == nil {
				// Common VictoriaLogs stream fields
				for _, field := range []string{"job", "instance", "host", "app", "service", "level", "stream"} {
					if v, ok := entry[field]; ok {
						if s, ok := v.(string); ok && s != "" {
							streamLabels[field] = s
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read logsql response: %w", err)
	}

	// Build labels: merge stream labels with query info
	labels := make(map[string]string, len(streamLabels)+2)
	labels["__logsql__"] = "true"
	labels["query"] = expression
	for k, v := range streamLabels {
		labels[k] = v
	}

	return []QueryResult{
		{
			MetricName: "logsql_match_count",
			Labels:     labels,
			Values: []DataPoint{
				{Timestamp: now, Value: count},
			},
		},
	}, nil
}
