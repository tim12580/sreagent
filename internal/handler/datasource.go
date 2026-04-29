package handler

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/service"
)

type DataSourceHandler struct {
	svc *service.DataSourceService
}

func NewDataSourceHandler(svc *service.DataSourceService) *DataSourceHandler {
	return &DataSourceHandler{svc: svc}
}

// CreateDataSourceRequest is the request body for creating/updating a datasource.
type CreateDataSourceRequest struct {
	Name                string               `json:"name" binding:"required"`
	Type                model.DataSourceType `json:"type" binding:"required"`
	Endpoint            string               `json:"endpoint" binding:"required,url"`
	Description         string               `json:"description"`
	Labels              model.JSONLabels     `json:"labels"`
	AuthType            string               `json:"auth_type"`
	AuthConfig          string               `json:"auth_config"`
	HealthCheckInterval int                  `json:"health_check_interval"`
	IsEnabled           *bool                `json:"is_enabled"`
}

// Create creates a new datasource.
func (h *DataSourceHandler) Create(c *gin.Context) {
	var req CreateDataSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	ds := &model.DataSource{
		Name:                req.Name,
		Type:                req.Type,
		Endpoint:            req.Endpoint,
		Description:         req.Description,
		Labels:              req.Labels,
		AuthType:            req.AuthType,
		AuthConfig:          req.AuthConfig,
		HealthCheckInterval: req.HealthCheckInterval,
		IsEnabled:           true,
	}

	if err := h.svc.Create(c.Request.Context(), ds); err != nil {
		Error(c, err)
		return
	}

	Success(c, ds)
}

// Get returns a single datasource by ID.
func (h *DataSourceHandler) Get(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	ds, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, ds)
}

// List returns a paginated list of datasources.
func (h *DataSourceHandler) List(c *gin.Context) {
	pq := GetPageQuery(c)
	dsType := c.Query("type")

	list, total, err := h.svc.List(c.Request.Context(), dsType, pq.Page, pq.PageSize)
	if err != nil {
		Error(c, err)
		return
	}

	SuccessPage(c, list, total, pq.Page, pq.PageSize)
}

// Update updates a datasource.
func (h *DataSourceHandler) Update(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req CreateDataSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	ds := &model.DataSource{
		Name:                req.Name,
		Type:                req.Type,
		Endpoint:            req.Endpoint,
		Description:         req.Description,
		Labels:              req.Labels,
		AuthType:            req.AuthType,
		AuthConfig:          req.AuthConfig,
		HealthCheckInterval: req.HealthCheckInterval,
	}
	if req.IsEnabled != nil {
		ds.IsEnabled = *req.IsEnabled
	}
	ds.ID = id

	if err := h.svc.Update(c.Request.Context(), ds); err != nil {
		Error(c, err)
		return
	}

	Success(c, ds)
}

// Delete deletes a datasource.
func (h *DataSourceHandler) Delete(c *gin.Context) {
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

// HealthCheck triggers a health check for a datasource.
func (h *DataSourceHandler) HealthCheck(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	result, err := h.svc.HealthCheck(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, result)
}

// Query tests an expression against a datasource.
// POST /api/v1/datasources/:id/query
func (h *DataSourceHandler) Query(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		Expression string  `json:"expression" binding:"required"`
		Time       float64 `json:"time"` // unix timestamp in seconds, 0 or omitted = now
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	var queryTime time.Time
	if req.Time > 0 {
		queryTime = time.UnixMilli(int64(req.Time * 1000))
	}

	result, err := h.svc.QueryDatasource(c.Request.Context(), id, req.Expression, queryTime)
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, result)
}

// RangeQuery executes a PromQL range query against a datasource.
// POST /api/v1/datasources/:id/query-range
func (h *DataSourceHandler) RangeQuery(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		Expression string  `json:"expression" binding:"required"`
		Start      float64 `json:"start" binding:"required"` // unix timestamp in seconds
		End        float64 `json:"end" binding:"required"`   // unix timestamp in seconds
		Step       string  `json:"step" binding:"required"`  // e.g. "15s", "1m", "5m"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	start := time.Unix(int64(req.Start), 0)
	end := time.Unix(int64(req.End), 0)

	result, err := h.svc.QueryRange(c.Request.Context(), id, req.Expression, start, end, req.Step)
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, result)
}

// LabelKeys returns label names from the target datasource (for PromQL autocompletion).
// GET /api/v1/datasources/:id/labels/keys
func (h *DataSourceHandler) LabelKeys(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	body, err := h.svc.ProxyToDatasource(c.Request.Context(), id, "/api/v1/labels", nil)
	if err != nil {
		Error(c, err)
		return
	}

	var apiResp struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		ErrorWithMessage(c, 50003, "failed to parse label keys response")
		return
	}

	Success(c, apiResp.Data)
}

// LabelValues returns values for a given label key from the target datasource.
// GET /api/v1/datasources/:id/labels/values?key=job
func (h *DataSourceHandler) LabelValues(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	key := c.Query("key")
	if key == "" {
		ErrorWithMessage(c, 10001, "key parameter is required")
		return
	}

	body, err := h.svc.ProxyToDatasource(c.Request.Context(), id, "/api/v1/label/"+key+"/values", nil)
	if err != nil {
		Error(c, err)
		return
	}

	var apiResp struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		ErrorWithMessage(c, 50003, "failed to parse label values response")
		return
	}

	Success(c, apiResp.Data)
}

// LogQuery executes a LogsQL query against a VictoriaLogs datasource and returns log entries.
// POST /api/v1/datasources/:id/log-query
func (h *DataSourceHandler) LogQuery(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	var req struct {
		Expression string  `json:"expression" binding:"required"`
		Start      float64 `json:"start" binding:"required"` // unix timestamp in seconds
		End        float64 `json:"end" binding:"required"`   // unix timestamp in seconds
		Limit      int     `json:"limit"`                    // max entries, default 100
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	start := time.Unix(int64(req.Start), 0)
	end := time.Unix(int64(req.End), 0)

	result, err := h.svc.QueryLogs(c.Request.Context(), id, service.LogQueryParams{
		Expression: req.Expression,
		Start:      start,
		End:        end,
		Limit:      req.Limit,
	})
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, result)
}

// MetricNames returns metric names from the target datasource (for PromQL autocompletion).
// GET /api/v1/datasources/:id/metrics?search=http&limit=100
func (h *DataSourceHandler) MetricNames(c *gin.Context) {
	id, err := GetIDParam(c, "id")
	if err != nil {
		Error(c, err)
		return
	}

	params := map[string]string{
		"label_name": "__name__",
	}
	if search := c.Query("search"); search != "" {
		params["search"] = search
	}
	if limit := c.Query("limit"); limit != "" {
		params["limit"] = limit
	}

	body, err := h.svc.ProxyToDatasource(c.Request.Context(), id, "/api/v1/label/__name__/values", params)
	if err != nil {
		Error(c, err)
		return
	}

	var apiResp struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		ErrorWithMessage(c, 50003, "failed to parse metric names response")
		return
	}

	Success(c, apiResp.Data)
}
