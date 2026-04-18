package handler

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sreagent/sreagent/internal/service"
)

// SMTPSettingsHandler manages global SMTP configuration.
type SMTPSettingsHandler struct {
	svc *service.SystemSettingService
}

// NewSMTPSettingsHandler creates a new SMTPSettingsHandler.
func NewSMTPSettingsHandler(svc *service.SystemSettingService) *SMTPSettingsHandler {
	return &SMTPSettingsHandler{svc: svc}
}

// GetConfig returns the current global SMTP configuration with password masked.
func (h *SMTPSettingsHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetSMTPConfig(c.Request.Context())
	if err != nil {
		Error(c, err)
		return
	}
	if cfg.Password != "" {
		cfg.Password = "********"
	}
	Success(c, cfg)
}

// UpdateConfig saves global SMTP configuration.
// Sending password = "********" preserves the existing password.
func (h *SMTPSettingsHandler) UpdateConfig(c *gin.Context) {
	var req service.SMTPConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}
	if req.Password == "********" {
		req.Password = ""
	}
	if err := h.svc.SaveSMTPConfig(c.Request.Context(), req); err != nil {
		Error(c, err)
		return
	}
	Success(c, nil)
}

// TestConnection sends a test email using the stored SMTP config.
// POST /settings/smtp/test   body: {"to": "user@example.com"}
func (h *SMTPSettingsHandler) TestConnection(c *gin.Context) {
	var req struct {
		To string `json:"to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithMessage(c, 10001, err.Error())
		return
	}

	cfg, err := h.svc.GetSMTPConfig(c.Request.Context())
	if err != nil {
		Error(c, err)
		return
	}
	if !cfg.Enabled || cfg.SMTPHost == "" {
		ErrorWithMessage(c, 10002, "SMTP is not configured or disabled")
		return
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 587
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + req.To,
		"Subject: SREAgent SMTP Test",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"This is a test email from SREAgent. Your SMTP configuration is working correctly.",
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, from, []string{req.To}, []byte(msg)); err != nil {
		ErrorWithMessage(c, 10002, "SMTP test failed: "+err.Error())
		return
	}

	Success(c, gin.H{"message": "Test email sent successfully to " + req.To})
}
