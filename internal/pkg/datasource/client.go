package datasource

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HealthResult carries the outcome of a detailed health probe.
type HealthResult struct {
	// Healthy is true when all probes pass.
	Healthy bool `json:"healthy"`
	// Message is a human-readable summary (version, error detail, etc.).
	Message string `json:"message"`
	// LatencyMs is the round-trip time of the query probe in milliseconds.
	// -1 means the probe was not reached (e.g. the connectivity check already failed).
	LatencyMs int64 `json:"latency_ms"`
	// Version is populated when the datasource exposes a version string.
	Version string `json:"version,omitempty"`
}

// HealthChecker is the interface for checking datasource health.
// CheckHealth returns a HealthResult — callers treat Healthy==false as unhealthy.
type HealthChecker interface {
	CheckHealth(ctx context.Context, endpoint, authType, authConfig string) HealthResult
}

// httpClient is a shared HTTP client with reasonable timeouts.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// NewChecker creates the appropriate health checker for a datasource type.
func NewChecker(dsType string) (HealthChecker, error) {
	switch dsType {
	case "prometheus":
		return &PrometheusChecker{}, nil
	case "victoriametrics":
		return &VictoriaMetricsChecker{}, nil
	case "zabbix":
		return &ZabbixChecker{}, nil
	case "victorialogs":
		return &VictoriaLogsChecker{}, nil
	default:
		return nil, fmt.Errorf("unsupported datasource type: %s", dsType)
	}
}
