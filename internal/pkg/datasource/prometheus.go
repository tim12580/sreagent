package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PrometheusChecker checks Prometheus API health.
type PrometheusChecker struct{}

// CheckHealth performs a two-phase probe:
//  1. GET /-/ready  — verifies the server is ready to serve queries (stricter than /-/healthy)
//  2. GET /api/v1/query?query=1 — verifies the PromQL engine is functional
//
// It also attempts to read the version from /api/v1/status/buildinfo.
func (c *PrometheusChecker) CheckHealth(ctx context.Context, endpoint, authType, authConfig string) HealthResult {
	base := strings.TrimRight(endpoint, "/")

	// ── Phase 1: readiness probe ──────────────────────────────────────────────
	if err := httpGet(ctx, base+"/-/ready", authType, authConfig); err != nil {
		return HealthResult{Healthy: false, LatencyMs: -1,
			Message: fmt.Sprintf("readiness probe failed: %v", err)}
	}

	// ── Phase 2: PromQL engine probe ─────────────────────────────────────────
	start := time.Now()
	body, status, err := httpGetBody(ctx, base+"/api/v1/query?query=1", authType, authConfig)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return HealthResult{Healthy: false, LatencyMs: latency,
			Message: fmt.Sprintf("query API unreachable: %v", err)}
	}
	if status != http.StatusOK {
		return HealthResult{Healthy: false, LatencyMs: latency,
			Message: fmt.Sprintf("query API returned HTTP %d", status)}
	}
	var qr struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	_ = json.Unmarshal(body, &qr)
	if qr.Status != "success" {
		return HealthResult{Healthy: false, LatencyMs: latency,
			Message: fmt.Sprintf("PromQL engine error: %s", qr.Error)}
	}

	// ── Phase 3 (optional): read version ────────────────────────────────────
	version := prometheusVersion(ctx, base, authType, authConfig)

	msg := "Prometheus is healthy"
	if version != "" {
		msg = fmt.Sprintf("Prometheus %s is healthy", version)
	}
	return HealthResult{Healthy: true, LatencyMs: latency, Message: msg, Version: version}
}

func prometheusVersion(ctx context.Context, base, authType, authConfig string) string {
	body, status, err := httpGetBody(ctx, base+"/api/v1/status/buildinfo", authType, authConfig)
	if err != nil || status != http.StatusOK {
		return ""
	}
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Status != "success" {
		return ""
	}
	return resp.Data.Version
}

// ── shared HTTP helpers ───────────────────────────────────────────────────────

func httpGet(ctx context.Context, url, authType, authConfig string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	applyAuth(req, authType, authConfig)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func httpGetBody(ctx context.Context, url, authType, authConfig string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	applyAuth(req, authType, authConfig)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}
