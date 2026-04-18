package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/service"
)

// MuteRuleHandler handles mute rule API requests.
type MuteRuleHandler struct {
	svc      *service.MuteRuleService
	eventSvc *service.AlertEventService
}

// NewMuteRuleHandler creates a new MuteRuleHandler.
func NewMuteRuleHandler(svc *service.MuteRuleService) *MuteRuleHandler {
	return &MuteRuleHandler{svc: svc}
}

// SetAlertEventService injects the alert event service for the preview endpoint.
func (h *MuteRuleHandler) SetAlertEventService(svc *service.AlertEventService) {
	h.eventSvc = svc
}

// CreateMuteRuleRequest is the request body for creating a mute rule.
type CreateMuteRuleRequest struct {
	Name          string           `json:"name" binding:"required"`
	Description   string           `json:"description"`
	MatchLabels   model.JSONLabels `json:"match_labels"`
	Severities    string           `json:"severities"`
	StartTime     *time.Time       `json:"start_time"`
	EndTime       *time.Time       `json:"end_time"`
	PeriodicStart string           `json:"periodic_start"`
	PeriodicEnd   string           `json:"periodic_end"`
	DaysOfWeek    string           `json:"days_of_week"`
	Timezone      string           `json:"timezone"`
	IsEnabled     bool             `json:"is_enabled"`
	RuleIDs       string           `json:"rule_ids"`
}

// Create creates a new mute rule.
func (h *MuteRuleHandler) Create(c *gin.Context) {
	var req CreateMuteRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	tz := req.Timezone
	if tz == "" {
		tz = "Asia/Shanghai"
	}

	rule := &model.MuteRule{
		Name:          req.Name,
		Description:   req.Description,
		MatchLabels:   req.MatchLabels,
		Severities:    req.Severities,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		PeriodicStart: req.PeriodicStart,
		PeriodicEnd:   req.PeriodicEnd,
		DaysOfWeek:    req.DaysOfWeek,
		Timezone:      tz,
		CreatedBy:     GetCurrentUserID(c),
		IsEnabled:     req.IsEnabled,
		RuleIDs:       req.RuleIDs,
	}

	if err := h.svc.Create(c.Request.Context(), rule); err != nil {
		Error(c, err)
		return
	}

	Success(c, rule)
}

// Get returns a mute rule by ID.
func (h *MuteRuleHandler) Get(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	rule, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, rule)
}

// List returns a paginated list of mute rules.
func (h *MuteRuleHandler) List(c *gin.Context) {
	pq := GetPageQuery(c)

	list, total, err := h.svc.List(c.Request.Context(), pq.Page, pq.PageSize)
	if err != nil {
		Error(c, err)
		return
	}

	SuccessPage(c, list, total, pq.Page, pq.PageSize)
}

// Update updates an existing mute rule.
func (h *MuteRuleHandler) Update(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req CreateMuteRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	tz := req.Timezone
	if tz == "" {
		tz = "Asia/Shanghai"
	}

	rule := &model.MuteRule{
		Name:          req.Name,
		Description:   req.Description,
		MatchLabels:   req.MatchLabels,
		Severities:    req.Severities,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		PeriodicStart: req.PeriodicStart,
		PeriodicEnd:   req.PeriodicEnd,
		DaysOfWeek:    req.DaysOfWeek,
		Timezone:      tz,
		IsEnabled:     req.IsEnabled,
		RuleIDs:       req.RuleIDs,
	}
	rule.ID = id

	if err := h.svc.Update(c.Request.Context(), rule); err != nil {
		Error(c, err)
		return
	}

	Success(c, rule)
}

// Delete deletes a mute rule by ID.
func (h *MuteRuleHandler) Delete(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		Error(c, err)
		return
	}

	Success(c, nil)
}

// MutePreviewItem describes which currently-firing alerts a mute rule would suppress.
type MutePreviewItem struct {
	RuleID        uint          `json:"rule_id"`
	RuleName      string        `json:"rule_name"`
	MatchedCount  int           `json:"matched_count"`
	MatchedAlerts []model.AlertEvent `json:"matched_alerts"`
}

// Preview returns a preview of which currently-firing alerts each enabled mute rule
// would suppress right now.
// GET /api/v1/mute-rules/preview
func (h *MuteRuleHandler) Preview(c *gin.Context) {
	if h.eventSvc == nil {
		ErrorWithMessage(c, 50000, "alert event service not available")
		return
	}

	ctx := c.Request.Context()

	// Fetch all enabled mute rules
	rules, _, err := h.svc.List(ctx, 1, 1000)
	if err != nil {
		Error(c, err)
		return
	}

	// Fetch all currently firing alerts (up to 500)
	firingEvents, _, err := h.eventSvc.List(ctx, "firing", "", 1, 500)
	if err != nil {
		Error(c, err)
		return
	}

	now := time.Now()
	result := make([]MutePreviewItem, 0, len(rules))
	for _, rule := range rules {
		if !rule.IsEnabled {
			continue
		}
		item := MutePreviewItem{
			RuleID:        rule.ID,
			RuleName:      rule.Name,
			MatchedAlerts: []model.AlertEvent{},
		}
		for _, ev := range firingEvents {
			if h.svc.MatchesRule(&rule, &ev, now) {
				item.MatchedAlerts = append(item.MatchedAlerts, ev)
			}
		}
		item.MatchedCount = len(item.MatchedAlerts)
		result = append(result, item)
	}

	Success(c, result)
}
