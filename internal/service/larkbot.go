package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/pkg/lark"
)

// LarkBotService handles Lark bot event callbacks and commands.
// Configuration is loaded from the DB via SystemSettingService on every call,
// so changes made in the Web UI take effect immediately without a restart.
type LarkBotService struct {
	settingSvc  *SystemSettingService
	eventSvc    *AlertEventService
	scheduleSvc *ScheduleService
	client      *http.Client
	logger      *zap.Logger
}

// NewLarkBotService creates a new LarkBotService backed by DB-stored configuration.
func NewLarkBotService(settingSvc *SystemSettingService, eventSvc *AlertEventService, scheduleSvc *ScheduleService, logger *zap.Logger) *LarkBotService {
	return &LarkBotService{
		settingSvc:  settingSvc,
		eventSvc:    eventSvc,
		scheduleSvc: scheduleSvc,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// loadConfig fetches the current Lark config from the DB.
func (s *LarkBotService) loadConfig(ctx context.Context) (LarkConfig, error) {
	return s.settingSvc.GetLarkConfig(ctx)
}

// GetConfig returns the current Lark bot configuration with secrets masked.
func (s *LarkBotService) GetConfig(ctx context.Context) (LarkConfig, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return LarkConfig{}, err
	}
	// Mask secrets for display
	if cfg.AppSecret != "" {
		if len(cfg.AppSecret) > 8 {
			cfg.AppSecret = cfg.AppSecret[:4] + "****" + cfg.AppSecret[len(cfg.AppSecret)-4:]
		} else {
			cfg.AppSecret = "****"
		}
	}
	if cfg.EncryptKey != "" {
		cfg.EncryptKey = "****"
	}
	if cfg.VerificationToken != "" {
		cfg.VerificationToken = "****"
	}
	return cfg, nil
}

// UpdateConfig persists the Lark bot configuration to the DB.
func (s *LarkBotService) UpdateConfig(ctx context.Context, cfg LarkConfig) error {
	return s.settingSvc.SaveLarkConfig(ctx, cfg)
}

// LarkEventRequest represents the incoming Lark event callback payload.
type LarkEventRequest struct {
	// URL verification fields
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`

	// Event subscription fields
	Schema string           `json:"schema"`
	Header *LarkEventHeader `json:"header"`
	Event  *LarkEventBody   `json:"event"`
}

// LarkEventHeader is the header part of a Lark event.
type LarkEventHeader struct {
	EventID    string `json:"event_id"`
	Token      string `json:"token"`
	CreateTime string `json:"create_time"`
	EventType  string `json:"event_type"`
	TenantKey  string `json:"tenant_key"`
	AppID      string `json:"app_id"`
}

// LarkEventBody is the event body for im.message.receive_v1.
type LarkEventBody struct {
	Sender  *LarkSender  `json:"sender"`
	Message *LarkMessage `json:"message"`
}

// LarkSender represents the message sender.
type LarkSender struct {
	SenderID   *LarkSenderID `json:"sender_id"`
	SenderType string        `json:"sender_type"`
	TenantKey  string        `json:"tenant_key"`
}

// LarkSenderID contains various ID formats for the sender.
type LarkSenderID struct {
	UnionID string `json:"union_id"`
	UserID  string `json:"user_id"`
	OpenID  string `json:"open_id"`
}

// LarkMessage represents the message content.
type LarkMessage struct {
	MessageID   string        `json:"message_id"`
	RootID      string        `json:"root_id"`
	ParentID    string        `json:"parent_id"`
	CreateTime  string        `json:"create_time"`
	ChatID      string        `json:"chat_id"`
	ChatType    string        `json:"chat_type"`
	MessageType string        `json:"message_type"`
	Content     string        `json:"content"`
	Mentions    []LarkMention `json:"mentions"`
}

// LarkMention represents an @mention in the message.
type LarkMention struct {
	Key       string        `json:"key"`
	ID        *LarkSenderID `json:"id"`
	Name      string        `json:"name"`
	TenantKey string        `json:"tenant_key"`
}

// HandleEvent processes a Lark event callback.
// Returns (response body, error).
func (s *LarkBotService) HandleEvent(ctx context.Context, body []byte) (interface{}, error) {
	// Parse the JSON body first — this is a cheap operation and must not be
	// blocked behind a DB call.  Loading config is only needed for token
	// verification which happens after parsing.
	var req LarkEventRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}

	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load lark config: %w", err)
	}

	// Handle URL verification challenge
	if req.Type == "url_verification" {
		if cfg.VerificationToken != "" && req.Token != cfg.VerificationToken {
			return nil, fmt.Errorf("invalid verification token")
		}
		return map[string]string{"challenge": req.Challenge}, nil
	}

	// Verify event token
	if cfg.VerificationToken != "" && req.Header != nil && req.Header.Token != cfg.VerificationToken {
		return nil, fmt.Errorf("invalid event token")
	}

	// Handle message events
	if req.Header != nil && req.Header.EventType == "im.message.receive_v1" {
		if err := s.handleMessageEvent(ctx, &req); err != nil {
			s.logger.Error("failed to handle message event", zap.Error(err))
			return nil, err
		}
	}

	return map[string]string{"status": "ok"}, nil
}

// handleMessageEvent processes a received message event.
func (s *LarkBotService) handleMessageEvent(ctx context.Context, req *LarkEventRequest) error {
	if req.Event == nil || req.Event.Message == nil {
		return nil
	}

	msg := req.Event.Message
	chatID := msg.ChatID
	userID := ""
	if req.Event.Sender != nil && req.Event.Sender.SenderID != nil {
		userID = req.Event.Sender.SenderID.OpenID
	}

	// Parse message content (Lark sends content as JSON string)
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(msg.Content), &content); err != nil {
		s.logger.Warn("failed to parse message content", zap.Error(err))
		return nil
	}

	// Strip @bot mentions from the text
	text := content.Text
	for _, mention := range msg.Mentions {
		text = strings.ReplaceAll(text, mention.Key, "")
	}
	text = strings.TrimSpace(text)

	// Parse command and args
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return s.SendMessage(ctx, chatID, "Please send a command. Available commands: /health, /oncall, /ack, /status")
	}

	command := parts[0]
	args := parts[1:]

	return s.HandleCommand(ctx, command, args, chatID, userID)
}

// HandleCommand routes and executes bot commands.
func (s *LarkBotService) HandleCommand(ctx context.Context, command string, args []string, chatID, userID string) error {
	switch command {
	case "/health":
		return s.cmdHealth(ctx, args, chatID)
	case "/oncall":
		return s.cmdOnCall(ctx, chatID)
	case "/ack":
		return s.cmdAck(ctx, args, chatID, userID)
	case "/status":
		return s.cmdStatus(ctx, chatID)
	default:
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Unknown command: %s\nAvailable commands: /health <cluster>, /oncall, /ack <alert_id>, /status", command))
	}
}

// cmdHealth handles the /health <cluster> command.
func (s *LarkBotService) cmdHealth(ctx context.Context, args []string, chatID string) error {
	cluster := ""
	if len(args) > 0 {
		cluster = args[0]
	}

	events, _, err := s.eventSvc.List(ctx, "firing", "", 1, 1000)
	if err != nil {
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Failed to fetch cluster health: %v", err))
	}

	// Filter by cluster label when specified
	var clusterAlerts int
	criticalCount := 0
	warningCount := 0
	for _, e := range events {
		if cluster != "" {
			labels := e.Labels
			if labels != nil {
				if c, ok := labels["cluster"]; !ok || c != cluster {
					continue
				}
			} else {
				continue
			}
		}
		clusterAlerts++
		switch strings.ToLower(string(e.Severity)) {
		case "critical":
			criticalCount++
		case "warning":
			warningCount++
		}
	}

	clusterLabel := cluster
	if clusterLabel == "" {
		clusterLabel = "all clusters"
	}

	var status string
	if criticalCount > 0 {
		status = "CRITICAL"
	} else if warningCount > 0 {
		status = "DEGRADED"
	} else if clusterAlerts > 0 {
		status = "WARNING"
	} else {
		status = "HEALTHY"
	}

	msg := fmt.Sprintf("Cluster Health: %s\n- Status: %s\n- Firing Alerts: %d\n- Critical: %d\n- Warning: %d",
		clusterLabel, status, clusterAlerts, criticalCount, warningCount)
	return s.SendMessage(ctx, chatID, msg)
}

// cmdOnCall handles the /oncall command.
func (s *LarkBotService) cmdOnCall(ctx context.Context, chatID string) error {
	if s.scheduleSvc == nil {
		return s.SendMessage(ctx, chatID, "On-call schedules are not configured.")
	}

	user, err := s.scheduleSvc.GetCurrentOnCallForAlert(ctx, map[string]string{})
	if err != nil || user == nil {
		return s.SendMessage(ctx, chatID, "No on-call user found. Please configure schedules in SREAgent.")
	}

	msg := fmt.Sprintf("Current On-Call:\n- Name: %s\n- Email: %s", user.DisplayName, user.Email)
	if user.Phone != "" {
		msg += fmt.Sprintf("\n- Phone: %s", user.Phone)
	}
	return s.SendMessage(ctx, chatID, msg)
}

// cmdAck handles the /ack <alert_id> command.
// Uses a system operator (ID=1) since Lark OpenID→User mapping is not implemented.
func (s *LarkBotService) cmdAck(ctx context.Context, args []string, chatID, userID string) error {
	if len(args) == 0 {
		return s.SendMessage(ctx, chatID, "Usage: /ack <alert_id>")
	}

	idStr := args[0]
	alertID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Invalid alert ID: %s. Please provide a numeric alert ID.", idStr))
	}

	// Use system user ID=1 as operator since Lark OpenID→DB User mapping is not configured
	const systemUserID = 1
	if err := s.eventSvc.Acknowledge(ctx, uint(alertID), systemUserID); err != nil {
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Failed to acknowledge alert #%d: %v", alertID, err))
	}

	return s.SendMessage(ctx, chatID, fmt.Sprintf("Alert #%d has been acknowledged.", alertID))
}

// cmdStatus handles the /status command.
func (s *LarkBotService) cmdStatus(ctx context.Context, chatID string) error {
	_, firingTotal, err := s.eventSvc.List(ctx, "firing", "", 1, 1)
	if err != nil {
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Failed to fetch alert status: %v", err))
	}

	_, criticalTotal, err := s.eventSvc.List(ctx, "firing", "critical", 1, 1)
	if err != nil {
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Failed to fetch critical alerts: %v", err))
	}
	_, warningTotal, err := s.eventSvc.List(ctx, "firing", "warning", 1, 1)
	if err != nil {
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Failed to fetch warning alerts: %v", err))
	}
	_, ackedTotal, err := s.eventSvc.List(ctx, "acknowledged", "", 1, 1)
	if err != nil {
		return s.SendMessage(ctx, chatID, fmt.Sprintf("Failed to fetch acknowledged alerts: %v", err))
	}

	msg := fmt.Sprintf("SREAgent Platform Status:\n- Active Alerts: %d\n- Critical: %d\n- Warning: %d\n- Acknowledged: %d",
		firingTotal, criticalTotal, warningTotal, ackedTotal)
	return s.SendMessage(ctx, chatID, msg)
}

// SendMessage sends a text message to a Lark chat.
//
// Routing preference:
//  1. If AppID + AppSecret are configured, use the Bot API to reply into the
//     originating chat (chatID), so @bot commands land in the correct room.
//  2. Otherwise fall back to the legacy incoming webhook (DefaultWebhook) — this
//     ignores chatID and is only useful for one-way broadcast setups.
func (s *LarkBotService) SendMessage(ctx context.Context, chatID, content string) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		s.logger.Warn("lark bot: failed to load config", zap.Error(err))
		return fmt.Errorf("failed to load lark config: %w", err)
	}

	// Preferred path: Bot API with chat_id (correct routing for command replies).
	if cfg.AppID != "" && cfg.AppSecret != "" && chatID != "" {
		bot := lark.NewBotClient(cfg.AppID, cfg.AppSecret)
		if _, err := bot.SendText(ctx, "chat_id", chatID, content); err != nil {
			s.logger.Warn("lark bot: Bot API send failed",
				zap.String("chat_id", chatID), zap.Error(err))
			return fmt.Errorf("lark bot API send failed: %w", err)
		}
		s.logger.Debug("lark bot text reply sent via Bot API", zap.String("chat_id", chatID))
		return nil
	}

	// Fallback: incoming webhook (chatID is ignored by webhook targets).
	if cfg.DefaultWebhook == "" {
		s.logger.Warn("lark bot: neither Bot API credentials nor default webhook configured")
		return fmt.Errorf("lark bot not configured (need AppID/AppSecret or DefaultWebhook)")
	}

	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DefaultWebhook, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send lark message: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read lark response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lark API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	s.logger.Debug("lark bot message sent", zap.String("chat_id", chatID))
	return nil
}
