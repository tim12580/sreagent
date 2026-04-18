package handler

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/repository"
	"github.com/sreagent/sreagent/internal/service"
)

type AlertEventHandler struct {
	svc      *service.AlertEventService
	auditSvc *service.AuditLogService
}

func NewAlertEventHandler(svc *service.AlertEventService) *AlertEventHandler {
	return &AlertEventHandler{svc: svc}
}

func (h *AlertEventHandler) SetAuditService(svc *service.AuditLogService) {
	h.auditSvc = svc
}

// List returns paginated alert events with optional filters.
// Supports view_mode=mine|unassigned|all and user_id for role-based visibility.
func (h *AlertEventHandler) List(c *gin.Context) {
	pq := GetPageQuery(c)

	filter := repository.AlertEventFilter{
		Status:   c.Query("status"),
		Severity: c.Query("severity"),
		ViewMode: c.Query("view_mode"),
		Page:     pq.Page,
		PageSize: pq.PageSize,
	}

	// user_id param overrides current user (admin use); default to current user
	if uidStr := c.Query("user_id"); uidStr != "" {
		if uid, err := strconv.ParseUint(uidStr, 10, 64); err == nil {
			filter.UserID = uint(uid)
		}
	}
	if filter.UserID == 0 {
		filter.UserID = GetCurrentUserID(c)
	}

	list, total, err := h.svc.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		Error(c, err)
		return
	}

	SuccessPage(c, list, total, pq.Page, pq.PageSize)
}

// Get returns a single alert event with its timeline.
func (h *AlertEventHandler) Get(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	event, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, event)
}

// Acknowledge marks an alert as acknowledged by the current user.
func (h *AlertEventHandler) Acknowledge(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	userID := GetCurrentUserID(c)
	if err := h.svc.Acknowledge(c.Request.Context(), id, userID); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(&model.AuditLog{
			UserID: &userID, Username: GetCurrentUsername(c),
			Action: model.AuditActionAck, ResourceType: model.AuditResourceAlertEvent,
			ResourceID: &id, IP: c.ClientIP(),
		})
	}
	Success(c, nil)
}

// Assign assigns an alert event to a specific user.
func (h *AlertEventHandler) Assign(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		AssignTo uint   `json:"assign_to" binding:"required"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	operatorID := GetCurrentUserID(c)
	if err := h.svc.Assign(c.Request.Context(), id, req.AssignTo, operatorID, req.Note); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(&model.AuditLog{
			UserID: &operatorID, Username: GetCurrentUsername(c),
			Action: model.AuditActionAssign, ResourceType: model.AuditResourceAlertEvent,
			ResourceID: &id, IP: c.ClientIP(),
		})
	}
	Success(c, nil)
}

// Resolve marks an alert as resolved.
func (h *AlertEventHandler) Resolve(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		Resolution string `json:"resolution"`
	}
	_ = c.ShouldBindJSON(&req)

	userID := GetCurrentUserID(c)
	if err := h.svc.Resolve(c.Request.Context(), id, userID, req.Resolution); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(&model.AuditLog{
			UserID: &userID, Username: GetCurrentUsername(c),
			Action: model.AuditActionResolve, ResourceType: model.AuditResourceAlertEvent,
			ResourceID: &id, IP: c.ClientIP(),
		})
	}
	Success(c, nil)
}

// Close closes an alert event.
func (h *AlertEventHandler) Close(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)

	userID := GetCurrentUserID(c)
	if err := h.svc.Close(c.Request.Context(), id, userID, req.Note); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(&model.AuditLog{
			UserID: &userID, Username: GetCurrentUsername(c),
			Action: model.AuditActionClose, ResourceType: model.AuditResourceAlertEvent,
			ResourceID: &id, IP: c.ClientIP(),
		})
	}
	Success(c, nil)
}

// AddComment adds a comment to an alert event timeline.
func (h *AlertEventHandler) AddComment(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	userID := GetCurrentUserID(c)
	if err := h.svc.AddComment(c.Request.Context(), id, userID, req.Note); err != nil {
		Error(c, err)
		return
	}

	Success(c, nil)
}

// GetTimeline returns the timeline for an alert event.
func (h *AlertEventHandler) GetTimeline(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	timeline, err := h.svc.GetTimeline(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, timeline)
}

// Silence silences an alert for a specified duration.
func (h *AlertEventHandler) Silence(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		DurationMinutes int    `json:"duration_minutes" binding:"required,min=1"`
		Reason          string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	userID := GetCurrentUserID(c)
	if err := h.svc.Silence(c.Request.Context(), id, userID, req.DurationMinutes, req.Reason); err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(&model.AuditLog{
			UserID: &userID, Username: GetCurrentUsername(c),
			Action: model.AuditActionSilence, ResourceType: model.AuditResourceAlertEvent,
			ResourceID: &id, IP: c.ClientIP(),
		})
	}
	Success(c, nil)
}

// BatchAcknowledge acknowledges multiple alerts at once.
func (h *AlertEventHandler) BatchAcknowledge(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	userID := GetCurrentUserID(c)
	success, failed, err := h.svc.BatchAcknowledge(c.Request.Context(), req.IDs, userID)
	if err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(&model.AuditLog{
			UserID: &userID, Username: GetCurrentUsername(c),
			Action: model.AuditActionAck, ResourceType: model.AuditResourceAlertEvent,
			Detail: "batch", IP: c.ClientIP(),
		})
	}
	Success(c, gin.H{"success": success, "failed": failed})
}

// BatchClose closes multiple alerts at once.
func (h *AlertEventHandler) BatchClose(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	userID := GetCurrentUserID(c)
	success, failed, err := h.svc.BatchClose(c.Request.Context(), req.IDs, userID)
	if err != nil {
		Error(c, err)
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Record(&model.AuditLog{
			UserID: &userID, Username: GetCurrentUsername(c),
			Action: model.AuditActionClose, ResourceType: model.AuditResourceAlertEvent,
			Detail: "batch", IP: c.ClientIP(),
		})
	}
	Success(c, gin.H{"success": success, "failed": failed})
}

// Export streams alert events as a CSV file.
// GET /api/v1/alert-events/export?status=firing&severity=critical&start=RFC3339&end=RFC3339
func (h *AlertEventHandler) Export(c *gin.Context) {
	filter := repository.AlertEventFilter{
		Status:   c.Query("status"),
		Severity: c.Query("severity"),
		Page:     1,
		PageSize: 10000, // cap at 10k rows
	}
	if uidStr := c.Query("user_id"); uidStr != "" {
		if uid, err := strconv.ParseUint(uidStr, 10, 64); err == nil {
			filter.UserID = uint(uid)
		}
	}
	filter.ViewMode = c.DefaultQuery("view_mode", "all")

	if startStr := c.Query("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartTime = &t
		}
	}
	if endStr := c.Query("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndTime = &t
		}
	}

	events, _, err := h.svc.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		Error(c, err)
		return
	}

	filename := fmt.Sprintf("alert-events-%s.csv", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Transfer-Encoding", "chunked")

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{
		"ID", "AlertName", "Severity", "Status", "Source",
		"FiredAt", "AckedAt", "ResolvedAt", "ClosedAt",
		"Labels", "Annotations", "Resolution", "FireCount",
	})

	fmtT := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Format(time.RFC3339)
	}
	fmtLabels := func(m model.JSONLabels) string {
		s := ""
		for k, v := range m {
			if s != "" {
				s += "; "
			}
			s += k + "=" + v
		}
		return s
	}

	for _, ev := range events {
		_ = w.Write([]string{
			strconv.FormatUint(uint64(ev.ID), 10),
			ev.AlertName,
			string(ev.Severity),
			string(ev.Status),
			ev.Source,
			ev.FiredAt.Format(time.RFC3339),
			fmtT(ev.AckedAt),
			fmtT(ev.ResolvedAt),
			fmtT(ev.ClosedAt),
			fmtLabels(ev.Labels),
			fmtLabels(ev.Annotations),
			ev.Resolution,
			strconv.Itoa(ev.FireCount),
		})
	}
	w.Flush()
}

// AlertGroupItem represents a set of alerts grouped by alert_name + source.
type AlertGroupItem struct {
	AlertName        string           `json:"alert_name"`
	Source           string           `json:"source"`
	TotalCount       int64            `json:"total_count"`
	SeverityBreakdown map[string]int64 `json:"severity_breakdown"`
	StatusBreakdown  map[string]int64 `json:"status_breakdown"`
	LatestFiredAt    time.Time        `json:"latest_fired_at"`
	OldestFiredAt    time.Time        `json:"oldest_fired_at"`
	MaxFireCount     int              `json:"max_fire_count"` // noisiest single event in group
}

// ListGroups aggregates alert events by alert_name + source so operators can
// spot noisy rules at a glance.
// GET /api/v1/alert-events/groups?status=firing,acknowledged&severity=critical,warning
func (h *AlertEventHandler) ListGroups(c *gin.Context) {
	// Status filter — default to active states
	statusParam := c.DefaultQuery("status", "firing,acknowledged,assigned")
	var statuses []string
	for _, s := range splitCSV(statusParam) {
		if s != "" {
			statuses = append(statuses, s)
		}
	}

	severityParam := c.Query("severity")
	var severities []string
	for _, s := range splitCSV(severityParam) {
		if s != "" {
			severities = append(severities, s)
		}
	}

	// Pull raw rows: one row per (alert_name, source, severity, status) combo.
	type rawRow struct {
		AlertName    string
		Source       string
		Severity     string
		Status       string
		Cnt          int64
		LatestFired  time.Time
		OldestFired  time.Time
		MaxFireCount int
	}
	q := h.svc.DB().Model(&model.AlertEvent{}).
		Select(`alert_name, source, severity, status,
			COUNT(*) AS cnt,
			MAX(fired_at) AS latest_fired,
			MIN(fired_at) AS oldest_fired,
			MAX(fire_count) AS max_fire_count`).
		Where("deleted_at IS NULL")

	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	if len(severities) > 0 {
		q = q.Where("severity IN ?", severities)
	}

	var rows []rawRow
	if err := q.Group("alert_name, source, severity, status").
		Order("latest_fired DESC").
		Scan(&rows).Error; err != nil {
		Error(c, err)
		return
	}

	// Merge into groups keyed by (alert_name, source).
	type key struct{ name, source string }
	order := []key{}
	groups := map[key]*AlertGroupItem{}

	for _, r := range rows {
		k := key{r.AlertName, r.Source}
		g, exists := groups[k]
		if !exists {
			g = &AlertGroupItem{
				AlertName:         r.AlertName,
				Source:            r.Source,
				SeverityBreakdown: map[string]int64{"critical": 0, "warning": 0, "info": 0},
				StatusBreakdown:   map[string]int64{},
				OldestFiredAt:     r.OldestFired,
				LatestFiredAt:     r.LatestFired,
			}
			groups[k] = g
			order = append(order, k)
		}
		g.TotalCount += r.Cnt
		g.SeverityBreakdown[r.Severity] += r.Cnt
		g.StatusBreakdown[r.Status] += r.Cnt
		if r.LatestFired.After(g.LatestFiredAt) {
			g.LatestFiredAt = r.LatestFired
		}
		if r.OldestFired.Before(g.OldestFiredAt) {
			g.OldestFiredAt = r.OldestFired
		}
		if r.MaxFireCount > g.MaxFireCount {
			g.MaxFireCount = r.MaxFireCount
		}
	}

	result := make([]AlertGroupItem, 0, len(order))
	for _, k := range order {
		result = append(result, *groups[k])
	}
	Success(c, result)
}

// splitCSV splits a comma-separated string into trimmed non-empty parts.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			p := s[start:i]
			if len(p) > 0 {
				parts = append(parts, p)
			}
			start = i + 1
		}
	}
	return parts
}

// WebhookReceive handles incoming alert webhooks (AlertManager compatible).
func (h *AlertEventHandler) WebhookReceive(c *gin.Context) {
	var payload model.AlertManagerPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	if err := h.svc.ProcessWebhook(c.Request.Context(), &payload); err != nil {
		Error(c, err)
		return
	}

	Success(c, nil)
}
