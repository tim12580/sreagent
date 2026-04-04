package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/service"
)

// OIDCSettingsHandler manages OIDC configuration stored in the DB.
// This is separate from OIDCHandler, which handles the actual SSO auth flow.
// Changes here require a pod restart to take effect (OIDC provider is initialized
// at startup); the UI warns the admin about this.
type OIDCSettingsHandler struct {
	settingSvc *service.SystemSettingService
}

// NewOIDCSettingsHandler creates a new OIDCSettingsHandler.
func NewOIDCSettingsHandler(settingSvc *service.SystemSettingService) *OIDCSettingsHandler {
	return &OIDCSettingsHandler{settingSvc: settingSvc}
}

// GetConfig returns the current OIDC configuration.
// The client_secret is masked if set (non-empty placeholder "********").
func (h *OIDCSettingsHandler) GetConfig(c *gin.Context) {
	cfg, err := h.settingSvc.GetOIDCConfig(c.Request.Context())
	if err != nil {
		ErrorWithMessage(c, 50003, "failed to load OIDC config: "+err.Error())
		return
	}
	// Mask the secret — never send it back to the browser.
	if cfg.ClientSecret != "" {
		cfg.ClientSecret = "********"
	}
	Success(c, cfg)
}

// UpdateConfig updates the OIDC configuration.
// If client_secret is empty or "********", the existing stored secret is preserved.
func (h *OIDCSettingsHandler) UpdateConfig(c *gin.Context) {
	var req service.OIDCConfigDB
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}
	// Treat the masked placeholder as "don't change the secret".
	if req.ClientSecret == "********" {
		req.ClientSecret = ""
	}
	if err := h.settingSvc.SaveOIDCConfig(c.Request.Context(), req); err != nil {
		ErrorWithMessage(c, 50003, "failed to save OIDC config: "+err.Error())
		return
	}
	Success(c, gin.H{"message": "OIDC configuration updated. Restart the pod to apply changes."})
}
