package router

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/config"
	"github.com/sreagent/sreagent/internal/handler"
	"github.com/sreagent/sreagent/internal/middleware"
)

// Handlers aggregates all handler instances.
type Handlers struct {
	Auth             *handler.AuthHandler
	OIDC             *handler.OIDCHandler // nil if OIDC is not configured
	OIDCSettings     *handler.OIDCSettingsHandler
	DataSource       *handler.DataSourceHandler
	AlertRule        *handler.AlertRuleHandler
	AlertEvent       *handler.AlertEventHandler
	Notification     *handler.NotificationHandler
	User             *handler.UserHandler
	Team             *handler.TeamHandler
	Schedule         *handler.ScheduleHandler
	Dashboard        *handler.DashboardHandler
	AI               *handler.AIHandler
	LarkBot          *handler.LarkBotHandler
	Engine           *handler.EngineHandler
	AlertAction      *handler.AlertActionHandler
	MuteRule         *handler.MuteRuleHandler
	NotifyRule       *handler.NotifyRuleHandler
	NotifyMedia      *handler.NotifyMediaHandler
	MessageTemplate  *handler.MessageTemplateHandler
	SubscribeRule    *handler.SubscribeRuleHandler
	BizGroup         *handler.BizGroupHandler
	AlertChannel     *handler.AlertChannelHandler
	UserNotifyConfig *handler.UserNotifyConfigHandler
	AuditLog         *handler.AuditLogHandler
	SMTPSettings     *handler.SMTPSettingsHandler
	SecuritySettings *handler.SecuritySettingsHandler
	InhibitionRule   *handler.InhibitionRuleHandler
	Heartbeat        *handler.HeartbeatHandler
	LabelRegistry    *handler.LabelRegistryHandler
}

// Setup initializes the Gin router with all routes and middleware.
func Setup(cfg *config.Config, handlers *Handlers, logger *zap.Logger) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.RequestLogger(logger))

	// Health check (no auth)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Webhook endpoint (no auth - authenticated by shared secret or source IP)
	webhooks := r.Group("/webhooks")
	{
		webhooks.POST("/alertmanager", handlers.AlertEvent.WebhookReceive)
	}

	// Heartbeat ping endpoint (no auth — token authenticates the sender)
	if handlers.Heartbeat != nil {
		r.POST("/heartbeat/:token", handlers.Heartbeat.Ping)
	}

	// Lark Bot callback (no auth - verified by token)
	r.POST("/lark/event", handlers.LarkBot.EventCallback)

	// Alert action page (no auth - token-based)
	if handlers.AlertAction != nil {
		r.GET("/alert-action/:token", handlers.AlertAction.ActionPage)
		r.POST("/alert-action/:token", handlers.AlertAction.ExecuteAction)
	}

	// API v1 routes
	api := r.Group("/api/v1")
	{
		// Public routes
		api.POST("/auth/login", handlers.Auth.Login)
		api.POST("/auth/refresh", handlers.Auth.Refresh)

		// OIDC routes (public — before JWT middleware)
		if handlers.OIDC != nil {
			api.GET("/auth/oidc/config", handlers.OIDC.OIDCConfig)
			api.GET("/auth/oidc/login", handlers.OIDC.LoginRedirect)
			api.GET("/auth/oidc/callback", handlers.OIDC.Callback)
			api.POST("/auth/oidc/token", handlers.OIDC.CallbackJSON)
		} else {
			// Return disabled status when OIDC is not configured
			api.GET("/auth/oidc/config", func(c *gin.Context) {
				c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{"enabled": false}})
			})
		}

		// ----- Authenticated routes (JWT required) -----
		auth := api.Group("")
		auth.Use(middleware.JWTAuth(&cfg.JWT))
		{
			// --- Role shorthand for readability ---
			adminOnly := middleware.RequireRole("admin")
			manage := middleware.RequireRole("admin", "team_lead")            // create/update/delete config objects
			operate := middleware.RequireRole("admin", "team_lead", "member") // operational actions (ack, resolve, etc.)
			// All authenticated users (viewer, global_viewer, member, team_lead, admin) can access GET/read routes
			// by virtue of passing JWTAuth without further RequireRole.

			// Current user (self) — any authenticated user
			auth.GET("/auth/profile", handlers.Auth.GetProfile)
			auth.PUT("/me/profile", handlers.Auth.UpdateMe)
			auth.POST("/me/password", handlers.Auth.ChangeMyPassword)
			auth.PUT("/me/lark-bind", handlers.Auth.BindLark)

			// DataSources
			ds := auth.Group("/datasources")
			{
				ds.GET("", handlers.DataSource.List)
				ds.GET("/:id", handlers.DataSource.Get)
				ds.POST("", adminOnly, handlers.DataSource.Create)
				ds.PUT("/:id", adminOnly, handlers.DataSource.Update)
				ds.DELETE("/:id", adminOnly, handlers.DataSource.Delete)
				ds.POST("/:id/health-check", manage, handlers.DataSource.HealthCheck)
				ds.POST("/:id/query", manage, handlers.DataSource.Query)
			}

			// Alert Rules
			rules := auth.Group("/alert-rules")
			{
				rules.GET("", handlers.AlertRule.List)
				rules.GET("/:id", handlers.AlertRule.Get)
				rules.GET("/categories", handlers.AlertRule.ListCategories)
				rules.GET("/export", handlers.AlertRule.Export)
				rules.POST("", manage, handlers.AlertRule.Create)
				rules.PUT("/:id", manage, handlers.AlertRule.Update)
				rules.DELETE("/:id", manage, handlers.AlertRule.Delete)
				rules.PATCH("/:id/status", manage, handlers.AlertRule.ToggleStatus)
				rules.POST("/import", manage, handlers.AlertRule.Import)
			}

			// Alert Events
			events := auth.Group("/alert-events")
			{
				events.GET("", handlers.AlertEvent.List)
				events.GET("/export", handlers.AlertEvent.Export)
				events.GET("/groups", handlers.AlertEvent.ListGroups)
				events.GET("/:id", handlers.AlertEvent.Get)
				events.GET("/:id/timeline", handlers.AlertEvent.GetTimeline)
				events.POST("/:id/acknowledge", operate, handlers.AlertEvent.Acknowledge)
				events.POST("/:id/assign", operate, handlers.AlertEvent.Assign)
				events.POST("/:id/resolve", operate, handlers.AlertEvent.Resolve)
				events.POST("/:id/close", operate, handlers.AlertEvent.Close)
				events.POST("/:id/comment", operate, handlers.AlertEvent.AddComment)
				events.POST("/:id/silence", operate, handlers.AlertEvent.Silence)
				events.POST("/batch/acknowledge", operate, handlers.AlertEvent.BatchAcknowledge)
				events.POST("/batch/close", operate, handlers.AlertEvent.BatchClose)
			}

			// Inhibition Rules
			if handlers.InhibitionRule != nil {
				inhibitions := auth.Group("/inhibition-rules")
				{
					inhibitions.GET("", handlers.InhibitionRule.List)
					inhibitions.GET("/:id", handlers.InhibitionRule.Get)
					inhibitions.POST("", manage, handlers.InhibitionRule.Create)
					inhibitions.PUT("/:id", manage, handlers.InhibitionRule.Update)
					inhibitions.DELETE("/:id", manage, handlers.InhibitionRule.Delete)
				}
			}

			// Mute Rules
			mutes := auth.Group("/mute-rules")
			{
				mutes.GET("", handlers.MuteRule.List)
				mutes.GET("/preview", handlers.MuteRule.Preview)
				mutes.GET("/:id", handlers.MuteRule.Get)
				mutes.POST("", manage, handlers.MuteRule.Create)
				mutes.PUT("/:id", manage, handlers.MuteRule.Update)
				mutes.DELETE("/:id", manage, handlers.MuteRule.Delete)
			}

			// Label Registry (autocomplete for match_labels)
			if handlers.LabelRegistry != nil {
				labelReg := auth.Group("/label-registry")
				{
					labelReg.GET("/keys", handlers.LabelRegistry.GetKeys)
					labelReg.GET("/values", handlers.LabelRegistry.GetValues)
					labelReg.POST("/sync", adminOnly, handlers.LabelRegistry.Sync)
				}
			}

			// Notify Rules (v2)
			notifyRules := auth.Group("/notify-rules")
			{
				notifyRules.GET("", handlers.NotifyRule.List)
				notifyRules.GET("/:id", handlers.NotifyRule.Get)
				notifyRules.POST("", manage, handlers.NotifyRule.Create)
				notifyRules.PUT("/:id", manage, handlers.NotifyRule.Update)
				notifyRules.DELETE("/:id", manage, handlers.NotifyRule.Delete)
			}

			// Notify Media
			notifyMedia := auth.Group("/notify-media")
			{
				notifyMedia.GET("", handlers.NotifyMedia.List)
				notifyMedia.GET("/:id", handlers.NotifyMedia.Get)
				notifyMedia.POST("", manage, handlers.NotifyMedia.Create)
				notifyMedia.PUT("/:id", manage, handlers.NotifyMedia.Update)
				notifyMedia.DELETE("/:id", manage, handlers.NotifyMedia.Delete)
				notifyMedia.POST("/:id/test", manage, handlers.NotifyMedia.Test)
			}

			// Message Templates
			templates := auth.Group("/message-templates")
			{
				templates.GET("", handlers.MessageTemplate.List)
				templates.GET("/:id", handlers.MessageTemplate.Get)
				templates.POST("", manage, handlers.MessageTemplate.Create)
				templates.PUT("/:id", manage, handlers.MessageTemplate.Update)
				templates.DELETE("/:id", manage, handlers.MessageTemplate.Delete)
				templates.POST("/preview", handlers.MessageTemplate.Preview)
			}

			// Subscribe Rules — members can manage their own subscriptions
			subscribes := auth.Group("/subscribe-rules")
			{
				subscribes.GET("", handlers.SubscribeRule.List)
				subscribes.GET("/:id", handlers.SubscribeRule.Get)
				subscribes.POST("", operate, handlers.SubscribeRule.Create)
				subscribes.PUT("/:id", operate, handlers.SubscribeRule.Update)
				subscribes.DELETE("/:id", operate, handlers.SubscribeRule.Delete)
			}

			// Business Groups
			bizGroups := auth.Group("/biz-groups")
			{
				bizGroups.GET("", handlers.BizGroup.List)
				bizGroups.GET("/tree", handlers.BizGroup.ListTree)
				bizGroups.GET("/:id", handlers.BizGroup.Get)
				bizGroups.GET("/:id/members", handlers.BizGroup.ListMembers)
				bizGroups.POST("", manage, handlers.BizGroup.Create)
				bizGroups.PUT("/:id", manage, handlers.BizGroup.Update)
				bizGroups.DELETE("/:id", manage, handlers.BizGroup.Delete)
				bizGroups.POST("/:id/members", manage, handlers.BizGroup.AddMember)
				bizGroups.DELETE("/:id/members/:uid", manage, handlers.BizGroup.RemoveMember)
			}

			// Alert Channels (virtual receivers)
			if handlers.AlertChannel != nil {
				alertChannels := auth.Group("/alert-channels")
				{
					alertChannels.GET("", handlers.AlertChannel.List)
					alertChannels.GET("/:id", handlers.AlertChannel.Get)
					alertChannels.POST("", manage, handlers.AlertChannel.Create)
					alertChannels.PUT("/:id", manage, handlers.AlertChannel.Update)
					alertChannels.DELETE("/:id", manage, handlers.AlertChannel.Delete)
					alertChannels.POST("/:id/test", manage, handlers.AlertChannel.Test)
				}
			}

			// User personal notify configs (multi-media, current user)
			if handlers.UserNotifyConfig != nil {
				auth.GET("/me/notify-configs", handlers.UserNotifyConfig.List)
				auth.PUT("/me/notify-configs", handlers.UserNotifyConfig.Upsert)
				auth.DELETE("/me/notify-configs/:mediaType", handlers.UserNotifyConfig.DeleteByMediaType)
			}

			// Notify Channels
			channels := auth.Group("/notify-channels")
			{
				channels.GET("", handlers.Notification.ListChannels)
				channels.GET("/:id", handlers.Notification.GetChannel)
				channels.POST("", manage, handlers.Notification.CreateChannel)
				channels.PUT("/:id", manage, handlers.Notification.UpdateChannel)
				channels.DELETE("/:id", manage, handlers.Notification.DeleteChannel)
				channels.POST("/:id/test", manage, handlers.Notification.TestChannel)
			}

			// Notify Policies
			policies := auth.Group("/notify-policies")
			{
				policies.GET("", handlers.Notification.ListPolicies)
				policies.GET("/:id", handlers.Notification.GetPolicy)
				policies.POST("", manage, handlers.Notification.CreatePolicy)
				policies.PUT("/:id", manage, handlers.Notification.UpdatePolicy)
				policies.DELETE("/:id", manage, handlers.Notification.DeletePolicy)
			}

			// Users — admin only for management
			users := auth.Group("/users")
			{
				users.GET("", handlers.User.List)
				users.GET("/:id", handlers.User.Get)
				users.POST("", adminOnly, handlers.User.Create)
				users.POST("/virtual", adminOnly, handlers.User.CreateVirtual)
				users.PUT("/:id", adminOnly, handlers.User.Update)
				users.PATCH("/:id/active", adminOnly, handlers.User.ToggleActive)
				users.PATCH("/:id/password", adminOnly, handlers.User.ChangePassword)
				users.DELETE("/:id", adminOnly, handlers.User.DeleteUser)
			}

			// Teams
			teams := auth.Group("/teams")
			{
				teams.GET("", handlers.Team.List)
				teams.GET("/:id", handlers.Team.Get)
				teams.GET("/:id/members", handlers.Team.ListMembers)
				teams.POST("", manage, handlers.Team.Create)
				teams.PUT("/:id", manage, handlers.Team.Update)
				teams.DELETE("/:id", manage, handlers.Team.Delete)
				teams.POST("/:id/members", manage, handlers.Team.AddMember)
				teams.DELETE("/:id/members/:uid", manage, handlers.Team.RemoveMember)
			}

			// Schedules
			schedules := auth.Group("/schedules")
			{
				schedules.GET("", handlers.Schedule.ListSchedules)
				schedules.GET("/:id", handlers.Schedule.GetSchedule)
				schedules.GET("/:id/oncall", handlers.Schedule.GetCurrentOnCall)
				schedules.GET("/:id/participants", handlers.Schedule.GetParticipants)
				schedules.GET("/:id/shifts", handlers.Schedule.ListShifts)
				schedules.POST("", manage, handlers.Schedule.CreateSchedule)
				schedules.PUT("/:id", manage, handlers.Schedule.UpdateSchedule)
				schedules.DELETE("/:id", manage, handlers.Schedule.DeleteSchedule)
				schedules.PUT("/:id/participants", manage, handlers.Schedule.SetParticipants)
				schedules.GET("/:id/overrides", handlers.Schedule.ListOverrides)
				schedules.POST("/:id/overrides", manage, handlers.Schedule.CreateOverride)
				schedules.DELETE("/:id/overrides/:oid", manage, handlers.Schedule.DeleteOverride)
				schedules.POST("/:id/shifts", manage, handlers.Schedule.CreateShift)
				schedules.PUT("/:id/shifts/:shiftId", manage, handlers.Schedule.UpdateShift)
				schedules.DELETE("/:id/shifts/:shiftId", manage, handlers.Schedule.DeleteShift)
				schedules.POST("/:id/generate-shifts", manage, handlers.Schedule.GenerateShifts)
				schedules.GET("/:id/ical", handlers.Schedule.ExportICal)
			}

			// Escalation Policies
			escalation := auth.Group("/escalation-policies")
			{
				escalation.GET("", handlers.Schedule.ListEscalationPolicies)
				escalation.GET("/:id", handlers.Schedule.GetEscalationPolicy)
				escalation.POST("", manage, handlers.Schedule.CreateEscalationPolicy)
				escalation.PUT("/:id", manage, handlers.Schedule.UpdateEscalationPolicy)
				escalation.DELETE("/:id", manage, handlers.Schedule.DeleteEscalationPolicy)
				escalation.POST("/:id/steps", manage, handlers.Schedule.CreateEscalationStep)
				escalation.PUT("/:id/steps/:stepId", manage, handlers.Schedule.UpdateEscalationStep)
				escalation.DELETE("/:id/steps/:stepId", manage, handlers.Schedule.DeleteEscalationStep)
			}

			// AI — config is admin only, usage is for all
			ai := auth.Group("/ai")
			{
				ai.POST("/alert-report", handlers.AI.GenerateReport)
				ai.POST("/suggest-sop", handlers.AI.SuggestSOP)
				ai.POST("/test", manage, handlers.AI.TestConnection)
				ai.GET("/config", adminOnly, handlers.AI.GetConfig)
				ai.PUT("/config", adminOnly, handlers.AI.UpdateConfig)
			}

			// Lark Bot config — admin only
			larkBot := auth.Group("/lark/bot")
			{
				larkBot.GET("/config", adminOnly, handlers.LarkBot.GetConfig)
				larkBot.PUT("/config", adminOnly, handlers.LarkBot.UpdateConfig)
			}

			// OIDC settings — admin only (separate from /auth/oidc/* which is the SSO auth flow)
			if handlers.OIDCSettings != nil {
				oidcSettings := auth.Group("/settings/oidc")
				{
					oidcSettings.GET("", adminOnly, handlers.OIDCSettings.GetConfig)
					oidcSettings.PUT("", adminOnly, handlers.OIDCSettings.UpdateConfig)
				}
			}

			// SMTP settings — admin only
			if handlers.SMTPSettings != nil {
				smtpSettings := auth.Group("/settings/smtp")
				{
					smtpSettings.GET("", adminOnly, handlers.SMTPSettings.GetConfig)
					smtpSettings.PUT("", adminOnly, handlers.SMTPSettings.UpdateConfig)
					smtpSettings.POST("/test", adminOnly, handlers.SMTPSettings.TestConnection)
				}
			}

			// Security settings — admin only
			if handlers.SecuritySettings != nil {
				secSettings := auth.Group("/settings/security")
				{
					secSettings.GET("", adminOnly, handlers.SecuritySettings.GetConfig)
					secSettings.PUT("", adminOnly, handlers.SecuritySettings.UpdateConfig)
				}
			}

			// Engine status (simple, no process management)
			if handlers.Engine != nil {
				auth.GET("/engine/status", handlers.Engine.GetStatus)
			}

			// Audit Logs — admin only
			auth.GET("/audit-logs", adminOnly, handlers.AuditLog.List)

			// Dashboard — all authenticated users
			auth.GET("/dashboard/stats", handlers.Dashboard.GetStats)
			auth.GET("/dashboard/mtta-mttr", handlers.Dashboard.GetMTTRStats)
			auth.GET("/dashboard/mttr-trend", handlers.Dashboard.GetMTTRTrend)
			auth.GET("/dashboard/alert-trend", handlers.Dashboard.GetAlertTrend)
			auth.GET("/dashboard/top-rules", handlers.Dashboard.GetTopRules)
			auth.GET("/dashboard/severity-history", handlers.Dashboard.GetSeverityHistory)
			auth.GET("/dashboard/export", handlers.Dashboard.ExportReport)
		}
	}

	// Serve frontend static files in production
	distPath := "web/dist"
	if _, err := os.Stat(distPath); err == nil {
		r.Static("/assets", path.Join(distPath, "assets"))
		r.StaticFile("/favicon.ico", path.Join(distPath, "favicon.ico"))
		r.StaticFile("/logo.svg", path.Join(distPath, "logo.svg"))

		r.NoRoute(func(c *gin.Context) {
			reqPath := c.Request.URL.Path
			// If it looks like a static file request, try to serve it
			if strings.Contains(reqPath, ".") {
				filePath := path.Join(distPath, reqPath)
				if _, err := os.Stat(filePath); err == nil {
					c.File(filePath)
					return
				}
				c.Status(http.StatusNotFound)
				return
			}
			// SPA fallback: serve index.html for all non-API routes
			c.File(path.Join(distPath, "index.html"))
		})
	}

	return r
}
