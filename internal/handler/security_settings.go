package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/config"
	"github.com/sreagent/sreagent/internal/service"
)

// SecuritySettingsHandler manages security-related platform settings.
type SecuritySettingsHandler struct {
	svc   *service.SystemSettingService
	jwtCfg *config.JWTConfig
}

// NewSecuritySettingsHandler creates a new SecuritySettingsHandler.
func NewSecuritySettingsHandler(svc *service.SystemSettingService, jwtCfg *config.JWTConfig) *SecuritySettingsHandler {
	return &SecuritySettingsHandler{svc: svc, jwtCfg: jwtCfg}
}

// GetConfig returns the current security configuration.
func (h *SecuritySettingsHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetSecurityConfig(c.Request.Context(), h.jwtCfg.Expire)
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, cfg)
}

// UpdateConfig saves security configuration.
func (h *SecuritySettingsHandler) UpdateConfig(c *gin.Context) {
	var req service.SecurityConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}
	if req.JWTExpireSeconds < 300 {
		ErrorWithMessage(c, 10001, "jwt_expire_seconds must be at least 300 (5 minutes)")
		return
	}
	if err := h.svc.SaveSecurityConfig(c.Request.Context(), req); err != nil {
		Error(c, err)
		return
	}
	Success(c, nil)
}
