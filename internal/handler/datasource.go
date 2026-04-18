package handler

import (
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
		Expression string `json:"expression" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	result, err := h.svc.QueryDatasource(c.Request.Context(), id, req.Expression)
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, result)
}
