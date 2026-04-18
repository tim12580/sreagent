package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// VictoriaMetricsChecker checks VictoriaMetrics health.
type VictoriaMetricsChecker struct{}

// CheckHealth performs a two-phase probe:
//  1. GET /health — basic liveness
//  2. GET /api/v1/query?query=1 — verifies the MetricsQL engine is operational
//
// Version is read from the X-Server-Hostname / response header or /api/v1/status/buildinfo.
func (c *VictoriaMetricsChecker) CheckHealth(ctx context.Context, endpoint, authType, authConfig string) HealthResult {
	base := strings.TrimRight(endpoint, "/")

	// ── Phase 1: liveness ────────────────────────────────────────────────────
	if err := httpGet(ctx, base+"/health", authType, authConfig); err != nil {
		return HealthResult{Healthy: false, LatencyMs: -1,
			Message: fmt.Sprintf("liveness probe failed: %v", err)}
	}

	// ── Phase 2: MetricsQL engine probe ──────────────────────────────────────
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
			Message: fmt.Sprintf("MetricsQL engine error: %s", qr.Error)}
	}

	// ── Phase 3 (optional): read version ─────────────────────────────────────
	version := vmVersion(ctx, base, authType, authConfig)

	msg := "VictoriaMetrics is healthy"
	if version != "" {
		msg = fmt.Sprintf("VictoriaMetrics %s is healthy", version)
	}
	return HealthResult{Healthy: true, LatencyMs: latency, Message: msg, Version: version}
}

func vmVersion(ctx context.Context, base, authType, authConfig string) string {
	// VictoriaMetrics exposes /api/v1/status/buildinfo similar to Prometheus.
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

// VictoriaMetricsInstantQuery executes a MetricsQL instant query.
// VictoriaMetrics is API-compatible with Prometheus, so we reuse QueryClient.InstantQuery.
func VictoriaMetricsInstantQuery(ctx context.Context, endpoint, authType, authConfig, expression string) ([]QueryResult, error) {
	return NewQueryClient().InstantQuery(ctx, endpoint, authType, authConfig, expression)
}
