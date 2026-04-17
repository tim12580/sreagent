package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/pkg/lark"
)

// LarkService wraps the Lark client for sending alert notifications.
type LarkService struct {
	client *lark.Client
	logger *zap.Logger
	// platformBaseURL is the base URL of the SREAgent web UI for deep-linking.
	platformBaseURL string
	// jwtSecret is used to sign alert action tokens.
	jwtSecret string
	// settingSvc provides Lark bot credentials for Bot API calls.
	settingSvc *SystemSettingService
}

// NewLarkService creates a new LarkService.
func NewLarkService(logger *zap.Logger, platformBaseURL, jwtSecret string) *LarkService {
	return &LarkService{
		client:          lark.NewClient(logger),
		logger:          logger,
		platformBaseURL: platformBaseURL,
		jwtSecret:       jwtSecret,
	}
}

// SetSystemSettingService injects the settings service for Bot API credential lookup.
func (s *LarkService) SetSystemSettingService(svc *SystemSettingService) {
	s.settingSvc = svc
}

// SendAlertNotification prepares and sends an alert notification via Lark webhook.
func (s *LarkService) SendAlertNotification(ctx context.Context, event *model.AlertEvent, webhookURL string) error {
	// Build the platform link for this alert event
	platformURL := ""
	if s.platformBaseURL != "" {
		platformURL = fmt.Sprintf("%s/alert-events/%d", s.platformBaseURL, event.ID)
	}

	card := lark.BuildAlertCard(
		event.AlertName,
		string(event.Severity),
		string(event.Status),
		event.Labels,
		event.Annotations,
		event.FiredAt,
		platformURL,
	)

	resp, err := s.client.SendWebhook(ctx, webhookURL, card)
	if err != nil {
		s.logger.Error("failed to send lark alert notification",
			zap.Uint("event_id", event.ID),
			zap.String("alert_name", event.AlertName),
			zap.Error(err),
		)
		return fmt.Errorf("lark webhook failed: %w", err)
	}

	s.logger.Info("lark alert notification sent",
		zap.Uint("event_id", event.ID),
		zap.String("alert_name", event.AlertName),
		zap.Int("resp_code", resp.Code),
	)
	return nil
}

// SendEnrichedAlertNotification sends an alert notification with AI analysis via Lark webhook.
func (s *LarkService) SendEnrichedAlertNotification(ctx context.Context, event *model.AlertEvent, analysis *AlertAnalysis, webhookURL string) error {
	// Build the platform link for this alert event
	platformURL := ""
	if s.platformBaseURL != "" {
		platformURL = fmt.Sprintf("%s/alert-events/%d", s.platformBaseURL, event.ID)
	}

	// Generate an action token for no-auth alert action page
	actionBaseURL := ""
	if s.platformBaseURL != "" && s.jwtSecret != "" {
		token, err := GenerateAlertActionToken(event.ID, s.jwtSecret)
		if err != nil {
			s.logger.Warn("failed to generate alert action token",
				zap.Uint("event_id", event.ID),
				zap.Error(err),
			)
		} else {
			actionBaseURL = fmt.Sprintf("%s/alert-action/%s", s.platformBaseURL, token)
		}
	}

	// Convert service.AlertAnalysis to lark.AIAnalysisResult (nil-safe)
	var aiResult *lark.AIAnalysisResult
	if analysis != nil {
		aiResult = &lark.AIAnalysisResult{
			Summary:          analysis.Summary,
			ProbableCauses:   analysis.ProbableCauses,
			Impact:           analysis.Impact,
			RecommendedSteps: analysis.RecommendedSteps,
		}
	}

	card := lark.BuildEnrichedAlertCard(
		event.AlertName,
		string(event.Severity),
		string(event.Status),
		event.Labels,
		event.Annotations,
		event.FiredAt,
		aiResult,
		platformURL,
		actionBaseURL,
	)

	resp, err := s.client.SendWebhook(ctx, webhookURL, card)
	if err != nil {
		s.logger.Error("failed to send enriched lark alert notification",
			zap.Uint("event_id", event.ID),
			zap.String("alert_name", event.AlertName),
			zap.Error(err),
		)
		return fmt.Errorf("lark webhook failed: %w", err)
	}

	s.logger.Info("enriched lark alert notification sent",
		zap.Uint("event_id", event.ID),
		zap.String("alert_name", event.AlertName),
		zap.Int("resp_code", resp.Code),
		zap.Bool("has_ai_analysis", analysis != nil),
	)
	return nil
}

// SendTestNotification sends a test card to the given webhook URL.
func (s *LarkService) SendTestNotification(ctx context.Context, webhookURL string) error {
	card := lark.BuildTestCard()

	_, err := s.client.SendWebhook(ctx, webhookURL, card)
	if err != nil {
		s.logger.Error("failed to send lark test notification", zap.Error(err))
		return fmt.Errorf("lark test webhook failed: %w", err)
	}

	s.logger.Info("lark test notification sent successfully")
	return nil
}

// SendEnrichedAlertNotificationViaBot sends an alert card via Lark Bot API to a group chat.
// Returns the message_id that can be used to update the card on status changes.
// chatID is the group's chat_id (e.g. "oc_xxxxx").
func (s *LarkService) SendEnrichedAlertNotificationViaBot(ctx context.Context, event *model.AlertEvent, analysis *AlertAnalysis, chatID string) (string, error) {
	if s.settingSvc == nil {
		return "", fmt.Errorf("settingSvc not configured for Bot API")
	}

	larkCfg, err := s.settingSvc.GetLarkConfig(ctx)
	if err != nil || larkCfg.AppID == "" || larkCfg.AppSecret == "" {
		return "", fmt.Errorf("lark bot credentials not configured")
	}

	card := s.buildEnrichedCard(event, analysis)
	botClient := lark.NewBotClient(larkCfg.AppID, larkCfg.AppSecret)

	msgID, err := botClient.SendMessage(ctx, chatID, card)
	if err != nil {
		s.logger.Error("failed to send alert card via Bot API",
			zap.Uint("event_id", event.ID), zap.Error(err))
		return "", fmt.Errorf("lark bot send failed: %w", err)
	}

	s.logger.Info("alert card sent via Bot API",
		zap.Uint("event_id", event.ID),
		zap.String("message_id", msgID),
	)
	return msgID, nil
}

// SendTestNotificationViaBot sends a test card to a Lark chat via Bot API (chat_id).
func (s *LarkService) SendTestNotificationViaBot(ctx context.Context, chatID string) error {
	if s.settingSvc == nil {
		return fmt.Errorf("settingSvc not configured for Bot API")
	}
	larkCfg, err := s.settingSvc.GetLarkConfig(ctx)
	if err != nil || larkCfg.AppID == "" || larkCfg.AppSecret == "" {
		return fmt.Errorf("lark bot credentials not configured")
	}
	card := lark.BuildTestCard()
	bot := lark.NewBotClient(larkCfg.AppID, larkCfg.AppSecret)
	if _, err := bot.SendMessage(ctx, chatID, card); err != nil {
		s.logger.Error("failed to send lark test card via Bot API",
			zap.String("chat_id", chatID), zap.Error(err))
		return fmt.Errorf("lark bot test send failed: %w", err)
	}
	s.logger.Info("lark test card sent via Bot API", zap.String("chat_id", chatID))
	return nil
}

// SendAlertCardToUser sends an enriched alert card directly to a Lark user (DM) via Bot API.
// receiveIDType is typically "user_id" (from UserNotifyConfig) or "open_id".
// Returns the message_id (not persisted to the event — DMs are per-recipient).
func (s *LarkService) SendAlertCardToUser(ctx context.Context, event *model.AlertEvent, analysis *AlertAnalysis, receiveIDType, receiveID string) (string, error) {
	if s.settingSvc == nil {
		return "", fmt.Errorf("settingSvc not configured for Bot API")
	}
	if receiveID == "" {
		return "", fmt.Errorf("receiveID is empty")
	}

	larkCfg, err := s.settingSvc.GetLarkConfig(ctx)
	if err != nil || larkCfg.AppID == "" || larkCfg.AppSecret == "" {
		return "", fmt.Errorf("lark bot credentials not configured")
	}

	card := s.buildEnrichedCard(event, analysis)
	botClient := lark.NewBotClient(larkCfg.AppID, larkCfg.AppSecret)

	msgID, err := botClient.SendDirectMessage(ctx, receiveIDType, receiveID, card)
	if err != nil {
		s.logger.Error("failed to send alert DM via Bot API",
			zap.Uint("event_id", event.ID),
			zap.String("receive_id_type", receiveIDType),
			zap.Error(err))
		return "", fmt.Errorf("lark bot DM failed: %w", err)
	}

	s.logger.Info("alert DM sent via Bot API",
		zap.Uint("event_id", event.ID),
		zap.String("receive_id_type", receiveIDType),
		zap.String("message_id", msgID),
	)
	return msgID, nil
}

// UpdateAlertCard patches the content of an existing card when the alert status changes.
// messageID is the value stored in alert_events.lark_message_id.
func (s *LarkService) UpdateAlertCard(ctx context.Context, event *model.AlertEvent, messageID string) error {
	if s.settingSvc == nil {
		return fmt.Errorf("settingSvc not configured for Bot API")
	}
	if messageID == "" {
		return nil // nothing to update
	}

	larkCfg, err := s.settingSvc.GetLarkConfig(ctx)
	if err != nil || larkCfg.AppID == "" || larkCfg.AppSecret == "" {
		return fmt.Errorf("lark bot credentials not configured")
	}

	card := s.buildEnrichedCard(event, nil)
	botClient := lark.NewBotClient(larkCfg.AppID, larkCfg.AppSecret)

	if err := botClient.UpdateMessage(ctx, messageID, card); err != nil {
		s.logger.Error("failed to update lark card",
			zap.Uint("event_id", event.ID),
			zap.String("message_id", messageID),
			zap.Error(err),
		)
		return fmt.Errorf("lark card update failed: %w", err)
	}

	s.logger.Info("lark card updated",
		zap.Uint("event_id", event.ID),
		zap.String("message_id", messageID),
		zap.String("new_status", string(event.Status)),
	)
	return nil
}

// buildEnrichedCard constructs the Lark interactive card for an alert event.
func (s *LarkService) buildEnrichedCard(event *model.AlertEvent, analysis *AlertAnalysis) *lark.CardMessage {
	platformURL := ""
	if s.platformBaseURL != "" {
		platformURL = fmt.Sprintf("%s/alert-events/%d", s.platformBaseURL, event.ID)
	}
	actionBaseURL := ""
	if s.platformBaseURL != "" && s.jwtSecret != "" {
		token, err := GenerateAlertActionToken(event.ID, s.jwtSecret)
		if err == nil {
			actionBaseURL = fmt.Sprintf("%s/alert-action/%s", s.platformBaseURL, token)
		}
	}

	var aiResult *lark.AIAnalysisResult
	if analysis != nil {
		aiResult = &lark.AIAnalysisResult{
			Summary:          analysis.Summary,
			ProbableCauses:   analysis.ProbableCauses,
			Impact:           analysis.Impact,
			RecommendedSteps: analysis.RecommendedSteps,
		}
	}

	return lark.BuildEnrichedAlertCard(
		event.AlertName,
		string(event.Severity),
		string(event.Status),
		event.Labels,
		event.Annotations,
		event.FiredAt,
		aiResult,
		platformURL,
		actionBaseURL,
	)
}
