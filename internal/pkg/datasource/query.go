package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DataPoint represents a single timestamped metric value.
type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// QueryResult represents a single metric time series returned by a PromQL query.
type QueryResult struct {
	MetricName string
	Labels     map[string]string
	Values     []DataPoint
}

// QueryClient executes PromQL queries against Prometheus-compatible APIs.
type QueryClient struct {
	httpClient *http.Client
}

// NewQueryClient creates a new QueryClient with sensible defaults.
func NewQueryClient() *QueryClient {
	return &QueryClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// prometheusAPIResponse is the top-level JSON response from Prometheus API.
type prometheusAPIResponse struct {
	Status string            `json:"status"`
	Error  string            `json:"error"`
	Data   prometheusAPIData `json:"data"`
}

// prometheusAPIData holds the result type and raw results.
type prometheusAPIData struct {
	ResultType string          `json:"resultType"`
	Result     json.RawMessage `json:"result"`
}

// prometheusSeries represents a single series in the Prometheus API response.
type prometheusSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"` // matrix: [[timestamp, value], ...]
	Value  []interface{}     `json:"value"`  // vector: [timestamp, value]
}

// RangeQuery executes a PromQL range query and returns parsed results.
func (qc *QueryClient) RangeQuery(ctx context.Context, endpoint, authType, authConfig, query string, start, end time.Time, step string) ([]QueryResult, error) {
	apiURL := strings.TrimRight(endpoint, "/") + "/api/v1/query_range"

	form := url.Values{}
	form.Set("query", query)
	form.Set("start", strconv.FormatInt(start.Unix(), 10))
	form.Set("end", strconv.FormatInt(end.Unix(), 10))
	form.Set("step", step)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create range query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	applyAuth(req, authType, authConfig)

	resp, err := qc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("range query request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read range query response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("range query returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseAPIResponse(body)
}

// InstantQuery executes a PromQL instant query and returns parsed results.
// If queryTime is zero, the server uses "now".
func (qc *QueryClient) InstantQuery(ctx context.Context, endpoint, authType, authConfig, query string, queryTime time.Time) ([]QueryResult, error) {
	apiURL := strings.TrimRight(endpoint, "/") + "/api/v1/query"

	form := url.Values{}
	form.Set("query", query)
	if !queryTime.IsZero() {
		form.Set("time", strconv.FormatFloat(float64(queryTime.UnixMilli())/1000, 'f', -1, 64))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create instant query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	applyAuth(req, authType, authConfig)

	resp, err := qc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instant query request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read instant query response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instant query returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseAPIResponse(body)
}

// parseAPIResponse parses a standard Prometheus API JSON response into QueryResults.
func parseAPIResponse(body []byte) ([]QueryResult, error) {
	var apiResp prometheusAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "success" {
		return nil, fmt.Errorf("API returned error: %s", apiResp.Error)
	}

	switch apiResp.Data.ResultType {
	case "matrix":
		return parseMatrixResult(apiResp.Data.Result)
	case "vector":
		return parseVectorResult(apiResp.Data.Result)
	default:
		return nil, fmt.Errorf("unsupported result type: %s", apiResp.Data.ResultType)
	}
}

// parseMatrixResult parses a matrix (range query) result.
func parseMatrixResult(raw json.RawMessage) ([]QueryResult, error) {
	var series []prometheusSeries
	if err := json.Unmarshal(raw, &series); err != nil {
		return nil, fmt.Errorf("failed to parse matrix result: %w", err)
	}

	results := make([]QueryResult, 0, len(series))
	for _, s := range series {
		qr := QueryResult{
			MetricName: s.Metric["__name__"],
			Labels:     s.Metric,
			Values:     make([]DataPoint, 0, len(s.Values)),
		}
		for _, v := range s.Values {
			dp, err := parseDataPoint(v)
			if err != nil {
				continue
			}
			qr.Values = append(qr.Values, dp)
		}
		results = append(results, qr)
	}

	return results, nil
}

// parseVectorResult parses a vector (instant query) result.
func parseVectorResult(raw json.RawMessage) ([]QueryResult, error) {
	var series []prometheusSeries
	if err := json.Unmarshal(raw, &series); err != nil {
		return nil, fmt.Errorf("failed to parse vector result: %w", err)
	}

	results := make([]QueryResult, 0, len(series))
	for _, s := range series {
		qr := QueryResult{
			MetricName: s.Metric["__name__"],
			Labels:     s.Metric,
		}
		if s.Value != nil {
			dp, err := parseDataPoint(s.Value)
			if err == nil {
				qr.Values = []DataPoint{dp}
			}
		}
		results = append(results, qr)
	}

	return results, nil
}

// parseDataPoint converts a Prometheus [timestamp, value] pair into a DataPoint.
func parseDataPoint(raw []interface{}) (DataPoint, error) {
	if len(raw) != 2 {
		return DataPoint{}, fmt.Errorf("expected 2 elements, got %d", len(raw))
	}

	// Timestamp is a float64 (unix seconds)
	tsFloat, ok := raw[0].(float64)
	if !ok {
		return DataPoint{}, fmt.Errorf("unexpected timestamp type: %T", raw[0])
	}
	ts := time.Unix(int64(tsFloat), 0).UTC()

	// Value is a string in Prometheus JSON responses
	valStr, ok := raw[1].(string)
	if !ok {
		return DataPoint{}, fmt.Errorf("unexpected value type: %T", raw[1])
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return DataPoint{}, fmt.Errorf("failed to parse value %q: %w", valStr, err)
	}

	return DataPoint{
		Timestamp: ts,
		Value:     val,
	}, nil
}
