package handler

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/sreagent/sreagent/internal/model"
)

type DashboardHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewDashboardHandler(db *gorm.DB, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{db: db, logger: logger}
}

// DashboardStats represents the aggregated dashboard statistics.
type DashboardStats struct {
	TotalDatasources  int64            `json:"total_datasources"`
	TotalRules        int64            `json:"total_rules"`
	ActiveAlerts      int64            `json:"active_alerts"`
	ResolvedToday     int64            `json:"resolved_today"`
	TotalUsers        int64            `json:"total_users"`
	TotalTeams        int64            `json:"total_teams"`
	// SeverityBreakdown holds the count of active (firing+acked) alerts per severity.
	SeverityBreakdown map[string]int64 `json:"severity_breakdown"`
}

// GetStats returns aggregated dashboard statistics.
func (h *DashboardHandler) GetStats(c *gin.Context) {
	var stats DashboardStats

	// Total datasources
	if err := h.db.Model(&model.DataSource{}).Count(&stats.TotalDatasources).Error; err != nil {
		h.logger.Error("failed to count datasources", zap.Error(err))
	}

	// Total alert rules
	if err := h.db.Model(&model.AlertRule{}).Count(&stats.TotalRules).Error; err != nil {
		h.logger.Error("failed to count alert rules", zap.Error(err))
	}

	// Active alerts (firing + acknowledged)
	if err := h.db.Model(&model.AlertEvent{}).
		Where("status IN ?", []string{
			string(model.EventStatusFiring),
			string(model.EventStatusAcknowledged),
		}).
		Count(&stats.ActiveAlerts).Error; err != nil {
		h.logger.Error("failed to count active alerts", zap.Error(err))
	}

	// Resolved today
	todayStart := time.Now().Truncate(24 * time.Hour)
	if err := h.db.Model(&model.AlertEvent{}).
		Where("status = ? AND resolved_at >= ?", string(model.EventStatusResolved), todayStart).
		Count(&stats.ResolvedToday).Error; err != nil {
		h.logger.Error("failed to count resolved alerts today", zap.Error(err))
	}

	// Total users
	if err := h.db.Model(&model.User{}).Count(&stats.TotalUsers).Error; err != nil {
		h.logger.Error("failed to count users", zap.Error(err))
	}

	// Total teams
	if err := h.db.Model(&model.Team{}).Count(&stats.TotalTeams).Error; err != nil {
		h.logger.Error("failed to count teams", zap.Error(err))
	}

	// Severity breakdown of active alerts
	type sevRow struct {
		Severity string
		Cnt      int64
	}
	var sevRows []sevRow
	h.db.Model(&model.AlertEvent{}).
		Select("severity, COUNT(*) AS cnt").
		Where("status IN ?", []string{
			string(model.EventStatusFiring),
			string(model.EventStatusAcknowledged),
		}).
		Group("severity").
		Scan(&sevRows)
	stats.SeverityBreakdown = map[string]int64{
		"critical": 0,
		"warning":  0,
		"info":     0,
	}
	for _, r := range sevRows {
		stats.SeverityBreakdown[r.Severity] = r.Cnt
	}

	Success(c, stats)
}

// MTTRMetric holds the mean, P50, and P95 of a latency distribution.
// All values are seconds; -1 means "no data in window".
type MTTRMetric struct {
	Mean  float64 `json:"mean"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	Count int64   `json:"count"`
}

// SeverityMTTR holds MTTA/MTTR for a single severity level.
type SeverityMTTR struct {
	Severity string     `json:"severity"`
	MTTA     MTTRMetric `json:"mtta"`
	MTTR     MTTRMetric `json:"mttr"`
}

// MTTRStats holds time-to-acknowledge and time-to-resolve statistics over a
// configurable window. Percentiles are computed in application code rather than
// with SQL percentile functions so we stay portable across MySQL versions.
type MTTRStats struct {
	WindowHours int `json:"window_hours"`

	// Overall (all severities combined).
	MTTA MTTRMetric `json:"mtta"`
	MTTR MTTRMetric `json:"mttr"`

	// Per-severity breakdown. Order is critical → warning → info.
	BySeverity []SeverityMTTR `json:"by_severity"`

	// Legacy fields retained for older dashboard builds. These mirror the
	// fields inside MTTA/MTTR above and should be phased out once the new
	// UI is deployed everywhere.
	MTTASeconds   float64 `json:"mtta_seconds"`
	MTTRSeconds   float64 `json:"mttr_seconds"`
	AckedCount    int64   `json:"acked_count"`
	ResolvedCount int64   `json:"resolved_count"`
}

// percentile returns the `p` percentile (0–100) of a slice of durations in
// seconds using nearest-rank. The input MUST be sorted ascending.
// Returns -1 when the slice is empty.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return -1
	}
	if n == 1 {
		return sorted[0]
	}
	// Nearest-rank: ceil(p/100 * n) — result index in [1, n].
	rank := int((p/100.0)*float64(n) + 0.9999999)
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

// computeMetric builds an MTTRMetric from an unsorted []float64 of seconds.
func computeMetric(durations []float64) MTTRMetric {
	n := len(durations)
	if n == 0 {
		return MTTRMetric{Mean: -1, P50: -1, P95: -1, Count: 0}
	}
	var sum float64
	for _, d := range durations {
		sum += d
	}
	sort.Float64s(durations)
	return MTTRMetric{
		Mean:  sum / float64(n),
		P50:   percentile(durations, 50),
		P95:   percentile(durations, 95),
		Count: int64(n),
	}
}

// GetMTTRStats returns MTTA and MTTR over a configurable window including
// percentiles and severity breakdown.
// GET /api/v1/dashboard/mttr-stats?hours=24
func (h *DashboardHandler) GetMTTRStats(c *gin.Context) {
	hours := 24
	if v := c.Query("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	// Pull raw durations so we can compute mean + percentiles in Go. This
	// avoids MySQL-version-specific percentile_disc / window tricks and
	// keeps the SQL uniform. A 30-day window on a busy tenant is bounded
	// by the number of fired events (typically <100k), which is fine to
	// stream into memory.
	type row struct {
		Severity    string
		AckSeconds  *float64
		RespSeconds *float64
	}

	var rows []row
	h.db.Model(&model.AlertEvent{}).
		Select(`severity,
			CASE WHEN acked_at    IS NOT NULL THEN TIMESTAMPDIFF(SECOND, fired_at, acked_at)    END AS ack_seconds,
			CASE WHEN resolved_at IS NOT NULL THEN TIMESTAMPDIFF(SECOND, fired_at, resolved_at) END AS resp_seconds`).
		Where("fired_at >= ? AND deleted_at IS NULL", since).
		Scan(&rows)

	var allAck, allResp []float64
	perSev := map[string]*struct{ ack, resp []float64 }{}

	for _, r := range rows {
		if r.AckSeconds != nil && *r.AckSeconds >= 0 {
			allAck = append(allAck, *r.AckSeconds)
		}
		if r.RespSeconds != nil && *r.RespSeconds >= 0 {
			allResp = append(allResp, *r.RespSeconds)
		}
		sev := r.Severity
		if sev == "" {
			continue
		}
		bucket, ok := perSev[sev]
		if !ok {
			bucket = &struct{ ack, resp []float64 }{}
			perSev[sev] = bucket
		}
		if r.AckSeconds != nil && *r.AckSeconds >= 0 {
			bucket.ack = append(bucket.ack, *r.AckSeconds)
		}
		if r.RespSeconds != nil && *r.RespSeconds >= 0 {
			bucket.resp = append(bucket.resp, *r.RespSeconds)
		}
	}

	stats := MTTRStats{
		WindowHours: hours,
		MTTA:        computeMetric(allAck),
		MTTR:        computeMetric(allResp),
	}

	// Deterministic severity ordering, most-critical first.
	for _, sev := range []string{"critical", "warning", "info"} {
		b, ok := perSev[sev]
		if !ok {
			b = &struct{ ack, resp []float64 }{}
		}
		stats.BySeverity = append(stats.BySeverity, SeverityMTTR{
			Severity: sev,
			MTTA:     computeMetric(b.ack),
			MTTR:     computeMetric(b.resp),
		})
	}

	// Legacy mirrors for older frontends.
	stats.MTTASeconds = stats.MTTA.Mean
	stats.MTTRSeconds = stats.MTTR.Mean
	stats.AckedCount = stats.MTTA.Count
	stats.ResolvedCount = stats.MTTR.Count

	Success(c, stats)
}

// MTTRTrendPoint is one day of MTTA/MTTR means used to render trend lines.
type MTTRTrendPoint struct {
	Date          string  `json:"date"`
	MTTASeconds   float64 `json:"mtta_seconds"`   // -1 if no data that day
	MTTRSeconds   float64 `json:"mttr_seconds"`   // -1 if no data that day
	AckedCount    int64   `json:"acked_count"`
	ResolvedCount int64   `json:"resolved_count"`
}

// GetMTTRTrend returns day-by-day MTTA/MTTR means so operators can see whether
// response times are improving or regressing over time.
// GET /api/v1/dashboard/mttr-trend?days=30
func (h *DashboardHandler) GetMTTRTrend(c *gin.Context) {
	days := 30
	if v := c.Query("days"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 && n <= 365 {
			days = n
		}
	}
	since := time.Now().AddDate(0, 0, -days)

	type ackRow struct {
		Date   string
		AvgSec float64
		Cnt    int64
	}

	var mttaRows []ackRow
	h.db.Model(&model.AlertEvent{}).
		Select(`DATE(fired_at) AS date,
			AVG(TIMESTAMPDIFF(SECOND, fired_at, acked_at)) AS avg_sec,
			COUNT(acked_at) AS cnt`).
		Where("fired_at >= ? AND acked_at IS NOT NULL AND deleted_at IS NULL", since).
		Group("DATE(fired_at)").
		Order("date").
		Scan(&mttaRows)

	var mttrRows []ackRow
	h.db.Model(&model.AlertEvent{}).
		Select(`DATE(fired_at) AS date,
			AVG(TIMESTAMPDIFF(SECOND, fired_at, resolved_at)) AS avg_sec,
			COUNT(resolved_at) AS cnt`).
		Where("fired_at >= ? AND resolved_at IS NOT NULL AND deleted_at IS NULL", since).
		Group("DATE(fired_at)").
		Order("date").
		Scan(&mttrRows)

	// Merge both sides into a single date-indexed map, producing a point
	// per calendar day that had *any* activity. Missing sides are emitted
	// as -1 so the chart can show gaps clearly.
	points := map[string]*MTTRTrendPoint{}
	for _, r := range mttaRows {
		p, ok := points[r.Date]
		if !ok {
			p = &MTTRTrendPoint{Date: r.Date, MTTASeconds: -1, MTTRSeconds: -1}
			points[r.Date] = p
		}
		p.MTTASeconds = r.AvgSec
		p.AckedCount = r.Cnt
	}
	for _, r := range mttrRows {
		p, ok := points[r.Date]
		if !ok {
			p = &MTTRTrendPoint{Date: r.Date, MTTASeconds: -1, MTTRSeconds: -1}
			points[r.Date] = p
		}
		p.MTTRSeconds = r.AvgSec
		p.ResolvedCount = r.Cnt
	}

	dates := make([]string, 0, len(points))
	for d := range points {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	result := make([]MTTRTrendPoint, 0, len(dates))
	for _, d := range dates {
		result = append(result, *points[d])
	}
	Success(c, result)
}

// AlertTrendPoint represents a data point for the alert trend chart.
type AlertTrendPoint struct {
	Date          string `json:"date"`
	FiredCount    int64  `json:"fired_count"`
	ResolvedCount int64  `json:"resolved_count"`
}

// GetAlertTrend returns daily fired/resolved counts for trend charts.
// GET /api/v1/dashboard/alert-trend?days=30
func (h *DashboardHandler) GetAlertTrend(c *gin.Context) {
	days := 30
	if v := c.Query("days"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 && n <= 365 {
			days = n
		}
	}
	since := time.Now().AddDate(0, 0, -days)

	type dateCount struct {
		Date string
		Cnt  int64
	}

	// Query fired counts per day
	var firedRows []dateCount
	h.db.Model(&model.AlertEvent{}).
		Select("DATE(fired_at) AS date, COUNT(*) AS cnt").
		Where("fired_at >= ? AND deleted_at IS NULL", since).
		Group("DATE(fired_at)").Order("date").Scan(&firedRows)

	// Query resolved counts per day
	var resolvedRows []dateCount
	h.db.Model(&model.AlertEvent{}).
		Select("DATE(resolved_at) AS date, COUNT(*) AS cnt").
		Where("resolved_at >= ? AND resolved_at IS NOT NULL AND deleted_at IS NULL", since).
		Group("DATE(resolved_at)").Order("date").Scan(&resolvedRows)

	// Merge into result
	resolvedMap := map[string]int64{}
	for _, r := range resolvedRows {
		resolvedMap[r.Date] = r.Cnt
	}

	result := make([]AlertTrendPoint, 0, len(firedRows))
	for _, f := range firedRows {
		result = append(result, AlertTrendPoint{
			Date: f.Date, FiredCount: f.Cnt, ResolvedCount: resolvedMap[f.Date],
		})
	}
	Success(c, result)
}

// TopRuleItem represents a rule with its alert count for the top-rules endpoint.
type TopRuleItem struct {
	RuleID    *uint  `json:"rule_id"`
	AlertName string `json:"alert_name"`
	Count     int64  `json:"count"`
}

// GetTopRules returns the most frequently firing alert rules.
// GET /api/v1/dashboard/top-rules?days=30&limit=10
func (h *DashboardHandler) GetTopRules(c *gin.Context) {
	days := 30
	if v := c.Query("days"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			days = n
		}
	}
	limit := 10
	if v := c.Query("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 && n <= 50 {
			limit = n
		}
	}
	since := time.Now().AddDate(0, 0, -days)

	var items []TopRuleItem
	h.db.Model(&model.AlertEvent{}).
		Select("rule_id, alert_name, COUNT(*) AS count").
		Where("fired_at >= ? AND deleted_at IS NULL", since).
		Group("rule_id, alert_name").
		Order("count DESC").
		Limit(limit).
		Scan(&items)
	Success(c, items)
}

// SeverityHistoryPoint represents per-severity alert counts for a single day.
type SeverityHistoryPoint struct {
	Date   string         `json:"date"`
	Counts map[string]int64 `json:"counts"`
}

// GetSeverityHistory returns daily alert counts broken down by severity.
// GET /api/v1/dashboard/severity-history?days=30
func (h *DashboardHandler) GetSeverityHistory(c *gin.Context) {
	days := 30
	if v := c.Query("days"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			days = n
		}
	}
	since := time.Now().AddDate(0, 0, -days)

	type row struct {
		Date     string
		Severity string
		Cnt      int64
	}
	var rows []row
	h.db.Model(&model.AlertEvent{}).
		Select("DATE(fired_at) AS date, severity, COUNT(*) AS cnt").
		Where("fired_at >= ? AND deleted_at IS NULL", since).
		Group("DATE(fired_at), severity").
		Order("date").
		Scan(&rows)

	dateMap := map[string]map[string]int64{}
	for _, r := range rows {
		if dateMap[r.Date] == nil {
			dateMap[r.Date] = map[string]int64{"critical": 0, "warning": 0, "info": 0}
		}
		dateMap[r.Date][r.Severity] = r.Cnt
	}

	// Sort dates
	dates := make([]string, 0, len(dateMap))
	for d := range dateMap {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	result := make([]SeverityHistoryPoint, 0, len(dates))
	for _, d := range dates {
		result = append(result, SeverityHistoryPoint{Date: d, Counts: dateMap[d]})
	}
	Success(c, result)
}

// ExportReport streams a CSV report covering daily alert counts and MTTA/MTTR
// for the requested date range (defaults to the last 30 days).
// GET /api/v1/dashboard/export?start_date=2006-01-02&end_date=2006-01-02
func (h *DashboardHandler) ExportReport(c *gin.Context) {
	// ── Parse date range ──────────────────────────────────────────────────
	const dateFmt = "2006-01-02"
	now := time.Now()
	endDate := now
	startDate := now.AddDate(0, 0, -29) // default: last 30 days

	if v := c.Query("start_date"); v != "" {
		if t, err := time.Parse(dateFmt, v); err == nil {
			startDate = t
		}
	}
	if v := c.Query("end_date"); v != "" {
		if t, err := time.Parse(dateFmt, v); err == nil {
			endDate = t
		}
	}
	if endDate.Before(startDate) {
		endDate = startDate
	}
	// Clamp to prevent accidental huge ranges (max 366 days)
	if endDate.Sub(startDate) > 366*24*time.Hour {
		startDate = endDate.AddDate(0, 0, -365)
	}

	startTS := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.Local)
	endTS := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, time.Local)

	// ── Per-day fired counts by severity ─────────────────────────────────
	type sevDayRow struct {
		Date     string
		Severity string
		Cnt      int64
	}
	var sevRows []sevDayRow
	h.db.Model(&model.AlertEvent{}).
		Select("DATE(fired_at) AS date, severity, COUNT(*) AS cnt").
		Where("fired_at BETWEEN ? AND ? AND deleted_at IS NULL", startTS, endTS).
		Group("DATE(fired_at), severity").
		Order("date").
		Scan(&sevRows)

	// ── Per-day resolved counts ───────────────────────────────────────────
	type dayCount struct {
		Date string
		Cnt  int64
	}
	var resolvedRows []dayCount
	h.db.Model(&model.AlertEvent{}).
		Select("DATE(resolved_at) AS date, COUNT(*) AS cnt").
		Where("resolved_at BETWEEN ? AND ? AND deleted_at IS NULL", startTS, endTS).
		Group("DATE(resolved_at)").
		Scan(&resolvedRows)

	// ── Per-day MTTA / MTTR (mean) ────────────────────────────────────────
	type ttaRow struct {
		Date   string
		AvgSec *float64
	}
	var mttaRows, mttrRows []ttaRow
	h.db.Model(&model.AlertEvent{}).
		Select("DATE(fired_at) AS date, AVG(TIMESTAMPDIFF(SECOND, fired_at, acked_at)) AS avg_sec").
		Where("fired_at BETWEEN ? AND ? AND acked_at IS NOT NULL AND deleted_at IS NULL", startTS, endTS).
		Group("DATE(fired_at)").Scan(&mttaRows)
	h.db.Model(&model.AlertEvent{}).
		Select("DATE(fired_at) AS date, AVG(TIMESTAMPDIFF(SECOND, fired_at, resolved_at)) AS avg_sec").
		Where("fired_at BETWEEN ? AND ? AND resolved_at IS NOT NULL AND deleted_at IS NULL", startTS, endTS).
		Group("DATE(fired_at)").Scan(&mttrRows)

	// ── Top rules in range ────────────────────────────────────────────────
	type topRuleRow struct {
		AlertName string
		Cnt       int64
		Critical  int64
		Warning   int64
		Info      int64
	}
	var topRows []topRuleRow
	h.db.Model(&model.AlertEvent{}).
		Select(`alert_name,
			COUNT(*) AS cnt,
			SUM(CASE WHEN severity='critical' THEN 1 ELSE 0 END) AS critical,
			SUM(CASE WHEN severity='warning'  THEN 1 ELSE 0 END) AS warning,
			SUM(CASE WHEN severity='info'     THEN 1 ELSE 0 END) AS info`).
		Where("fired_at BETWEEN ? AND ? AND deleted_at IS NULL", startTS, endTS).
		Group("alert_name").Order("cnt DESC").Limit(20).
		Scan(&topRows)

	// ── Merge into day-keyed maps ─────────────────────────────────────────
	type daySummary struct {
		Critical, Warning, Info, Resolved int64
		AvgMTTA, AvgMTTR                 float64
	}
	dayMap := map[string]*daySummary{}
	ensureDay := func(d string) *daySummary {
		if dayMap[d] == nil {
			dayMap[d] = &daySummary{AvgMTTA: -1, AvgMTTR: -1}
		}
		return dayMap[d]
	}
	for _, r := range sevRows {
		s := ensureDay(r.Date)
		switch r.Severity {
		case "critical":
			s.Critical = r.Cnt
		case "warning":
			s.Warning = r.Cnt
		case "info":
			s.Info = r.Cnt
		}
	}
	for _, r := range resolvedRows {
		ensureDay(r.Date).Resolved = r.Cnt
	}
	for _, r := range mttaRows {
		if r.AvgSec != nil {
			ensureDay(r.Date).AvgMTTA = *r.AvgSec / 60.0
		}
	}
	for _, r := range mttrRows {
		if r.AvgSec != nil {
			ensureDay(r.Date).AvgMTTR = *r.AvgSec / 60.0
		}
	}

	// Build sorted date list (fill every calendar day in range)
	var dates []string
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		key := d.Format(dateFmt)
		ensureDay(key) // ensure entry exists even for quiet days
		dates = append(dates, key)
	}
	sort.Strings(dates)

	// ── Stream CSV ────────────────────────────────────────────────────────
	fname := fmt.Sprintf("alert-report-%s-to-%s.csv",
		startDate.Format("20060102"), endDate.Format("20060102"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+fname)

	w := csv.NewWriter(c.Writer)

	// Section 1: daily summary
	_ = w.Write([]string{"# Daily Alert Summary"})
	_ = w.Write([]string{
		"Date", "Total", "Critical", "Warning", "Info",
		"Resolved", "Avg MTTA (min)", "Avg MTTR (min)",
	})
	fmtF := func(f float64) string {
		if f < 0 {
			return "-"
		}
		return fmt.Sprintf("%.1f", f)
	}
	for _, d := range dates {
		s := dayMap[d]
		total := s.Critical + s.Warning + s.Info
		_ = w.Write([]string{
			d,
			strconv.FormatInt(total, 10),
			strconv.FormatInt(s.Critical, 10),
			strconv.FormatInt(s.Warning, 10),
			strconv.FormatInt(s.Info, 10),
			strconv.FormatInt(s.Resolved, 10),
			fmtF(s.AvgMTTA),
			fmtF(s.AvgMTTR),
		})
	}

	// Section 2: top rules
	_ = w.Write([]string{})
	_ = w.Write([]string{"# Top Alert Rules"})
	_ = w.Write([]string{"Rule Name", "Total", "Critical", "Warning", "Info"})
	for _, r := range topRows {
		_ = w.Write([]string{
			r.AlertName,
			strconv.FormatInt(r.Cnt, 10),
			strconv.FormatInt(r.Critical, 10),
			strconv.FormatInt(r.Warning, 10),
			strconv.FormatInt(r.Info, 10),
		})
	}
	w.Flush()
}
