package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/model"
	apperr "github.com/sreagent/sreagent/internal/pkg/errors"
	"github.com/sreagent/sreagent/internal/repository"
)

// NotificationService is the notification routing engine.
type NotificationService struct {
	channelRepo   *repository.NotifyChannelRepository
	policyRepo    *repository.NotifyPolicyRepository
	recordRepo    *repository.NotifyRecordRepository
	eventRepo     *repository.AlertEventRepository
	larkSvc       *LarkService
	pipeline      *AlertPipeline
	subscribeSvc  *SubscribeRuleService
	notifyRuleSvc *NotifyRuleService
	logger        *zap.Logger
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(
	channelRepo *repository.NotifyChannelRepository,
	policyRepo *repository.NotifyPolicyRepository,
	recordRepo *repository.NotifyRecordRepository,
	larkSvc *LarkService,
	pipeline *AlertPipeline,
	logger *zap.Logger,
) *NotificationService {
	return &NotificationService{
		channelRepo: channelRepo,
		policyRepo:  policyRepo,
		recordRepo:  recordRepo,
		larkSvc:     larkSvc,
		pipeline:    pipeline,
		logger:      logger,
	}
}

// SetSubscribeRuleService sets the subscribe rule service for v2 notification pipeline.
// This uses setter injection to avoid circular dependency issues.
func (s *NotificationService) SetSubscribeRuleService(svc *SubscribeRuleService) {
	s.subscribeSvc = svc
}

// SetNotifyRuleService sets the notify rule service for v2 notification pipeline.
// This uses setter injection to avoid circular dependency issues.
func (s *NotificationService) SetNotifyRuleService(svc *NotifyRuleService) {
	s.notifyRuleSvc = svc
}

// SetAlertEventRepository injects the event repo so the notification service can
// persist lark_message_id after a successful Bot API send.
func (s *NotificationService) SetAlertEventRepository(repo *repository.AlertEventRepository) {
	s.eventRepo = repo
}

// RouteAlert is the main routing function. It finds matching policies by alert
// labels/severity, checks throttle, and dispatches notifications to channels.
func (s *NotificationService) RouteAlert(ctx context.Context, event *model.AlertEvent) error {
	// Skip notification for silenced alerts
	if event.Status == model.EventStatusSilenced && event.SilencedUntil != nil && event.SilencedUntil.After(time.Now()) {
		s.logger.Info("skipping notification for silenced alert",
			zap.Uint("event_id", event.ID),
			zap.Time("silenced_until", *event.SilencedUntil),
		)
		return nil
	}

	policies, err := s.policyRepo.FindMatchingPolicies(ctx, event.Labels, string(event.Severity))
	if err != nil {
		s.logger.Error("failed to find matching policies",
			zap.Uint("event_id", event.ID),
			zap.Error(err),
		)
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	if len(policies) == 0 {
		s.logger.Debug("no matching notification policies found",
			zap.Uint("event_id", event.ID),
			zap.String("alert_name", event.AlertName),
		)
		return nil
	}

	s.logger.Info("routing alert to notification channels",
		zap.Uint("event_id", event.ID),
		zap.String("alert_name", event.AlertName),
		zap.Int("matching_policies", len(policies)),
	)

	// Run AI analysis pipeline (async-safe, returns nil if AI disabled/fails)
	var analysis *AlertAnalysis
	if s.pipeline != nil {
		analysis = s.pipeline.AnalyzeAlert(ctx, event)
	}

	for _, policy := range policies {
		// Check throttle
		if s.isThrottled(ctx, &policy) {
			s.logger.Debug("notification throttled",
				zap.Uint("event_id", event.ID),
				zap.Uint("policy_id", policy.ID),
				zap.Int("throttle_minutes", policy.ThrottleMinutes),
			)
			// Record as throttled
			s.createRecord(ctx, event.ID, policy.ChannelID, policy.ID, "throttled", "")
			continue
		}

		// Send notification
		if err := s.SendNotification(ctx, event, &policy.Channel, &policy, analysis); err != nil {
			s.logger.Error("failed to send notification",
				zap.Uint("event_id", event.ID),
				zap.Uint("channel_id", policy.ChannelID),
				zap.Error(err),
			)
			s.createRecord(ctx, event.ID, policy.ChannelID, policy.ID, "failed", err.Error())
			continue
		}

		s.createRecord(ctx, event.ID, policy.ChannelID, policy.ID, "sent", "")
	}

	// --- V2 Subscription Pipeline ---
	// After the v1 policy-based pipeline, check for user/team subscribe rules
	// and dispatch through the v2 notify rule pipeline.
	s.processSubscriptions(ctx, event)

	return nil
}

// processSubscriptions finds matching subscribe rules and dispatches each
// through its associated notify rule. This bridges the v1 routing (policies)
// with the v2 pipeline (notify rules + media + templates).
func (s *NotificationService) processSubscriptions(ctx context.Context, event *model.AlertEvent) {
	if s.subscribeSvc == nil || s.notifyRuleSvc == nil {
		return
	}

	subscriptions, err := s.subscribeSvc.FindSubscriptions(ctx, event)
	if err != nil {
		s.logger.Error("failed to find matching subscriptions",
			zap.Uint("event_id", event.ID),
			zap.Error(err),
		)
		return
	}

	if len(subscriptions) == 0 {
		return
	}

	s.logger.Info("processing event through subscription rules",
		zap.Uint("event_id", event.ID),
		zap.Int("matching_subscriptions", len(subscriptions)),
	)

	for _, sub := range subscriptions {
		if sub.NotifyRuleID == 0 {
			s.logger.Debug("subscribe rule has no notify rule configured",
				zap.Uint("subscribe_rule_id", sub.ID),
			)
			continue
		}

		if err := s.notifyRuleSvc.ProcessEvent(ctx, event, sub.NotifyRuleID); err != nil {
			s.logger.Error("failed to process event through notify rule",
				zap.Uint("event_id", event.ID),
				zap.Uint("subscribe_rule_id", sub.ID),
				zap.Uint("notify_rule_id", sub.NotifyRuleID),
				zap.Error(err),
			)
		}
	}
}

// SendNotification sends a notification to a specific channel based on its type.
func (s *NotificationService) SendNotification(ctx context.Context, event *model.AlertEvent, channel *model.NotifyChannel, policy *model.NotifyPolicy, analysis *AlertAnalysis) error {
	switch channel.Type {
	case model.ChannelTypeLarkWebhook, model.ChannelTypeLarkBot:
		var larkCfg struct {
			WebhookURL string `json:"webhook_url"`
			ChatID     string `json:"chat_id"`
		}
		if err := json.Unmarshal([]byte(channel.Config), &larkCfg); err != nil {
			return fmt.Errorf("invalid channel config: %w", err)
		}
		if larkCfg.ChatID != "" {
			// Bot API path: returns message_id which enables in-place card updates
			msgID, err := s.larkSvc.SendEnrichedAlertNotificationViaBot(ctx, event, analysis, larkCfg.ChatID)
			if err != nil {
				return err
			}
			if msgID != "" && event.LarkMessageID == "" {
				event.LarkMessageID = msgID
				if s.eventRepo != nil {
					if dbErr := s.eventRepo.Update(ctx, event); dbErr != nil {
						s.logger.Warn("failed to persist lark_message_id",
							zap.Uint("event_id", event.ID),
							zap.Error(dbErr),
						)
					}
				}
			}
			return nil
		}
		if larkCfg.WebhookURL == "" {
			return fmt.Errorf("lark channel config must specify webhook_url or chat_id")
		}
		// Incoming Webhook path (no message_id, cards cannot be updated later)
		return s.larkSvc.SendEnrichedAlertNotification(ctx, event, analysis, larkCfg.WebhookURL)

	case model.ChannelTypeEmail:
		return s.sendEmailNotification(ctx, event, channel, analysis)

	case model.ChannelTypeCustom:
		return s.sendWebhookNotification(ctx, event, channel, analysis)

	default:
		s.logger.Warn("unsupported channel type",
			zap.String("type", string(channel.Type)),
			zap.Uint("channel_id", channel.ID),
		)
		return fmt.Errorf("unsupported channel type: %s", channel.Type)
	}
}

// isThrottled checks if a notification should be throttled based on the policy's throttle settings.
func (s *NotificationService) isThrottled(ctx context.Context, policy *model.NotifyPolicy) bool {
	if policy.ThrottleMinutes <= 0 {
		return false
	}

	lastRecord, err := s.recordRepo.GetLastSentRecord(ctx, policy.ChannelID, policy.ID)
	if err != nil {
		// No previous record found, not throttled
		return false
	}

	elapsed := time.Since(lastRecord.CreatedAt)
	throttleDuration := time.Duration(policy.ThrottleMinutes) * time.Minute
	return elapsed < throttleDuration
}

// createRecord creates a notification record for audit and tracking.
func (s *NotificationService) createRecord(ctx context.Context, eventID, channelID, policyID uint, status, response string) {
	record := &model.NotifyRecord{
		EventID:   eventID,
		ChannelID: channelID,
		PolicyID:  policyID,
		Status:    status,
		Response:  response,
	}
	if err := s.recordRepo.Create(ctx, record); err != nil {
		s.logger.Error("failed to create notify record",
			zap.Uint("event_id", eventID),
			zap.Error(err),
		)
	}
}

// --- Channel CRUD ---

// CreateChannel creates a new notification channel.
func (s *NotificationService) CreateChannel(ctx context.Context, channel *model.NotifyChannel) error {
	if err := s.channelRepo.Create(ctx, channel); err != nil {
		s.logger.Error("failed to create notify channel", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// GetChannel returns a notification channel by ID.
func (s *NotificationService) GetChannel(ctx context.Context, id uint) (*model.NotifyChannel, error) {
	channel, err := s.channelRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.ErrChannelNotFound
	}
	return channel, nil
}

// ListChannels returns a paginated list of notification channels.
func (s *NotificationService) ListChannels(ctx context.Context, page, pageSize int) ([]model.NotifyChannel, int64, error) {
	return s.channelRepo.List(ctx, page, pageSize)
}

// UpdateChannel updates an existing notification channel.
func (s *NotificationService) UpdateChannel(ctx context.Context, channel *model.NotifyChannel) error {
	existing, err := s.channelRepo.GetByID(ctx, channel.ID)
	if err != nil {
		return apperr.ErrChannelNotFound
	}

	existing.Name = channel.Name
	existing.Type = channel.Type
	existing.Description = channel.Description
	existing.Labels = channel.Labels
	if channel.Config != "" {
		existing.Config = channel.Config
	}
	existing.IsEnabled = channel.IsEnabled

	if err := s.channelRepo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to update notify channel", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// DeleteChannel deletes a notification channel by ID.
func (s *NotificationService) DeleteChannel(ctx context.Context, id uint) error {
	if _, err := s.channelRepo.GetByID(ctx, id); err != nil {
		return apperr.ErrChannelNotFound
	}

	if err := s.channelRepo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete notify channel", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// TestChannel sends a test notification to the specified channel.
func (s *NotificationService) TestChannel(ctx context.Context, channelID uint) error {
	channel, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return apperr.ErrChannelNotFound
	}

	switch channel.Type {
	case model.ChannelTypeLarkWebhook, model.ChannelTypeLarkBot:
		// Support both webhook-based channels and Bot API chat_id channels.
		var cfg struct {
			WebhookURL string `json:"webhook_url"`
			ChatID     string `json:"chat_id"`
		}
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return apperr.WithMessage(apperr.ErrBadRequest, "invalid channel config: "+err.Error())
		}
		if cfg.ChatID != "" {
			return s.larkSvc.SendTestNotificationViaBot(ctx, cfg.ChatID)
		}
		if cfg.WebhookURL == "" {
			return apperr.WithMessage(apperr.ErrBadRequest, "lark channel config must specify webhook_url or chat_id")
		}
		return s.larkSvc.SendTestNotification(ctx, cfg.WebhookURL)

	case model.ChannelTypeEmail:
		return s.testEmailChannel(ctx, channel)

	case model.ChannelTypeCustom:
		return s.testWebhookChannel(ctx, channel)

	default:
		return apperr.WithMessage(apperr.ErrBadRequest, fmt.Sprintf("test not supported for channel type: %s", channel.Type))
	}
}

// --- Policy CRUD ---

// CreatePolicy creates a new notification policy.
func (s *NotificationService) CreatePolicy(ctx context.Context, policy *model.NotifyPolicy) error {
	// Verify the target channel exists
	if _, err := s.channelRepo.GetByID(ctx, policy.ChannelID); err != nil {
		return apperr.ErrChannelNotFound
	}

	if err := s.policyRepo.Create(ctx, policy); err != nil {
		s.logger.Error("failed to create notify policy", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// GetPolicy returns a notification policy by ID.
func (s *NotificationService) GetPolicy(ctx context.Context, id uint) (*model.NotifyPolicy, error) {
	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.ErrPolicyNotFound
	}
	return policy, nil
}

// ListPolicies returns a paginated list of notification policies.
func (s *NotificationService) ListPolicies(ctx context.Context, page, pageSize int) ([]model.NotifyPolicy, int64, error) {
	return s.policyRepo.List(ctx, page, pageSize)
}

// UpdatePolicy updates an existing notification policy.
func (s *NotificationService) UpdatePolicy(ctx context.Context, policy *model.NotifyPolicy) error {
	existing, err := s.policyRepo.GetByID(ctx, policy.ID)
	if err != nil {
		return apperr.ErrPolicyNotFound
	}

	// Verify the target channel exists if it was changed
	if policy.ChannelID != existing.ChannelID {
		if _, err := s.channelRepo.GetByID(ctx, policy.ChannelID); err != nil {
			return apperr.ErrChannelNotFound
		}
	}

	existing.Name = policy.Name
	existing.Description = policy.Description
	existing.MatchLabels = policy.MatchLabels
	existing.Severities = policy.Severities
	existing.ChannelID = policy.ChannelID
	existing.ThrottleMinutes = policy.ThrottleMinutes
	existing.TemplateName = policy.TemplateName
	existing.IsEnabled = policy.IsEnabled
	existing.Priority = policy.Priority

	if err := s.policyRepo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to update notify policy", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// DeletePolicy deletes a notification policy by ID.
func (s *NotificationService) DeletePolicy(ctx context.Context, id uint) error {
	if _, err := s.policyRepo.GetByID(ctx, id); err != nil {
		return apperr.ErrPolicyNotFound
	}

	if err := s.policyRepo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete notify policy", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// channelConfig represents the JSON config of a notification channel.
type channelConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// extractWebhookURL extracts the webhook URL from a channel's JSON config.
func extractWebhookURL(config string) (string, error) {
	var cfg channelConfig
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return "", fmt.Errorf("failed to parse channel config: %w", err)
	}
	if cfg.WebhookURL == "" {
		return "", fmt.Errorf("webhook_url is empty in channel config")
	}
	return cfg.WebhookURL, nil
}

// --- Email (SMTP) channel ---

// emailChannelConfig is the JSON config for an email notification channel.
// Config JSON example:
//
//	{
//	  "smtp_host": "smtp.example.com",
//	  "smtp_port": 587,
//	  "smtp_tls":  true,
//	  "username":  "alert@example.com",
//	  "password":  "secret",
//	  "from":      "SREAgent <alert@example.com>",
//	  "recipients": ["ops@example.com", "team@example.com"]
//	}
type emailChannelConfig struct {
	SMTPHost   string   `json:"smtp_host"`
	SMTPPort   int      `json:"smtp_port"`
	SMTPTLS    bool     `json:"smtp_tls"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	From       string   `json:"from"`
	Recipients []string `json:"recipients"`
}

// buildEmailBody builds a plain-text email body for the alert event.
func buildEmailBody(event *model.AlertEvent, analysis *AlertAnalysis) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Alert: %s\n", event.AlertName))
	b.WriteString(fmt.Sprintf("Severity: %s\n", event.Severity))
	b.WriteString(fmt.Sprintf("Status: %s\n", event.Status))
	b.WriteString(fmt.Sprintf("Fired At: %s\n", event.FiredAt.Format(time.RFC3339)))
	if len(event.Labels) > 0 {
		b.WriteString("\nLabels:\n")
		for k, v := range event.Labels {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	if len(event.Annotations) > 0 {
		b.WriteString("\nAnnotations:\n")
		for k, v := range event.Annotations {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	if analysis != nil && analysis.Summary != "" {
		b.WriteString("\nAI Analysis:\n")
		b.WriteString(analysis.Summary)
		b.WriteString("\n")
	}
	return b.String()
}

// sendEmailNotification sends an alert notification via SMTP.
func (s *NotificationService) sendEmailNotification(_ context.Context, event *model.AlertEvent, channel *model.NotifyChannel, analysis *AlertAnalysis) error {
	var cfg emailChannelConfig
	if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
		return fmt.Errorf("invalid email channel config: %w", err)
	}
	if cfg.SMTPHost == "" {
		return fmt.Errorf("smtp_host is required in email channel config")
	}
	if len(cfg.Recipients) == 0 {
		return fmt.Errorf("recipients list is empty in email channel config")
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 587
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	subject := fmt.Sprintf("[%s] %s - %s", strings.ToUpper(string(event.Severity)), event.AlertName, event.Status)
	body := buildEmailBody(event, analysis)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from,
		strings.Join(cfg.Recipients, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	if cfg.SMTPTLS {
		// Use TLS (port 465 style) or STARTTLS on submission port
		tlsCfg := &tls.Config{ServerName: cfg.SMTPHost}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server with TLS: %w", err)
		}
		client, err := smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP authentication failed: %w", err)
			}
		}
		if err := client.Mail(cfg.Username); err != nil {
			return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
		}
		for _, to := range cfg.Recipients {
			if err := client.Rcpt(to); err != nil {
				return fmt.Errorf("SMTP RCPT TO %s failed: %w", to, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("SMTP DATA failed: %w", err)
		}
		_, err = w.Write([]byte(msg))
		if err != nil {
			return fmt.Errorf("SMTP write failed: %w", err)
		}
		return w.Close()
	}

	// Plain or STARTTLS (smtp.SendMail handles STARTTLS automatically)
	return smtp.SendMail(addr, auth, cfg.Username, cfg.Recipients, []byte(msg))
}

// testEmailChannel sends a test email via the configured SMTP channel.
func (s *NotificationService) testEmailChannel(_ context.Context, channel *model.NotifyChannel) error {
	var cfg emailChannelConfig
	if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
		return apperr.WithMessage(apperr.ErrBadRequest, "invalid email channel config: "+err.Error())
	}
	if cfg.SMTPHost == "" {
		return apperr.WithMessage(apperr.ErrBadRequest, "smtp_host is required")
	}
	if len(cfg.Recipients) == 0 {
		return apperr.WithMessage(apperr.ErrBadRequest, "recipients list is empty")
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 587
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: [SREAgent] Test Notification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nThis is a test notification from SREAgent.\r\n",
		from,
		strings.Join(cfg.Recipients, ", "),
	)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, cfg.Username, cfg.Recipients, []byte(msg)); err != nil {
		return apperr.WithMessage(apperr.ErrBadRequest, "SMTP test failed: "+err.Error())
	}
	return nil
}

// --- Custom Webhook (HTTP callback) channel ---

// customWebhookConfig is the JSON config for a custom HTTP webhook channel.
// Config JSON example:
//
//	{
//	  "url": "https://hooks.example.com/alert",
//	  "method": "POST",
//	  "headers": {"Authorization": "Bearer xxx", "X-Custom-Header": "value"},
//	  "timeout_seconds": 10
//	}
type customWebhookConfig struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	TimeoutSeconds int               `json:"timeout_seconds"`
}

// customWebhookPayload is the JSON body sent to a custom webhook.
type customWebhookPayload struct {
	EventID     uint              `json:"event_id"`
	AlertName   string            `json:"alert_name"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	FiredAt     string            `json:"fired_at"`
	Source      string            `json:"source"`
	AISummary   string            `json:"ai_summary,omitempty"`
}

// sendWebhookNotification sends an alert notification to a custom HTTP webhook.
func (s *NotificationService) sendWebhookNotification(_ context.Context, event *model.AlertEvent, channel *model.NotifyChannel, analysis *AlertAnalysis) error {
	var cfg customWebhookConfig
	if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
		return fmt.Errorf("invalid custom webhook config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("url is required in custom webhook config")
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodPost
	}
	timeoutSec := cfg.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 10
	}

	payload := customWebhookPayload{
		EventID:     event.ID,
		AlertName:   event.AlertName,
		Severity:    string(event.Severity),
		Status:      string(event.Status),
		Labels:      map[string]string(event.Labels),
		Annotations: map[string]string(event.Annotations),
		FiredAt:     event.FiredAt.Format(time.RFC3339),
		Source:      event.Source,
	}
	if analysis != nil {
		payload.AISummary = analysis.Summary
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	httpCli := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	req, err := http.NewRequest(cfg.Method, cfg.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	// Drain body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}

// testWebhookChannel sends a test payload to a custom HTTP webhook.
func (s *NotificationService) testWebhookChannel(_ context.Context, channel *model.NotifyChannel) error {
	var cfg customWebhookConfig
	if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
		return apperr.WithMessage(apperr.ErrBadRequest, "invalid custom webhook config: "+err.Error())
	}
	if cfg.URL == "" {
		return apperr.WithMessage(apperr.ErrBadRequest, "url is required")
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodPost
	}
	timeoutSec := cfg.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 10
	}

	payload := map[string]string{
		"source":  "SREAgent",
		"message": "This is a test notification from SREAgent.",
	}
	bodyBytes, _ := json.Marshal(payload)

	httpCli := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	req, err := http.NewRequest(cfg.Method, cfg.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return apperr.WithMessage(apperr.ErrBadRequest, "failed to create request: "+err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := httpCli.Do(req)
	if err != nil {
		return apperr.WithMessage(apperr.ErrBadRequest, "webhook test failed: "+err.Error())
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apperr.WithMessage(apperr.ErrBadRequest, fmt.Sprintf("webhook returned status %d", resp.StatusCode))
	}
	return nil
}
