package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/repository"
	"github.com/sreagent/sreagent/internal/service"
)

// EscalationExecutor periodically checks firing alert events and executes escalation steps
// when the configured delay has elapsed and the alert has not yet been resolved or acknowledged.
type EscalationExecutor struct {
	policyRepo          *repository.EscalationPolicyRepository
	stepRepo            *repository.EscalationStepRepository
	eventRepo           *repository.AlertEventRepository
	timelineRepo        *repository.AlertTimelineRepository
	channelRepo         *repository.NotifyChannelRepository
	userRepo            *repository.UserRepository
	notifySvc           *service.NotificationService
	userNotifyConfigRepo *repository.UserNotifyConfigRepository
	teamRepo            service.TeamRepository
	onCallShiftRepo     *repository.OnCallShiftRepository
	larkSvc             *service.LarkService // optional — enables lark_personal DM
	logger              *zap.Logger

	interval time.Duration
	stopCh   chan struct{}
	once     sync.Once
}

// SetLarkService injects a LarkService so the executor can deliver `lark_personal`
// escalation notifications as direct messages via the Lark Bot API.
func (e *EscalationExecutor) SetLarkService(svc *service.LarkService) {
	e.larkSvc = svc
}

// NewEscalationExecutor creates a new EscalationExecutor.
func NewEscalationExecutor(
	policyRepo *repository.EscalationPolicyRepository,
	stepRepo *repository.EscalationStepRepository,
	eventRepo *repository.AlertEventRepository,
	timelineRepo *repository.AlertTimelineRepository,
	channelRepo *repository.NotifyChannelRepository,
	userRepo *repository.UserRepository,
	notifySvc *service.NotificationService,
	userNotifyConfigRepo *repository.UserNotifyConfigRepository,
	teamRepo service.TeamRepository,
	onCallShiftRepo *repository.OnCallShiftRepository,
	logger *zap.Logger,
) *EscalationExecutor {
	return &EscalationExecutor{
		policyRepo:           policyRepo,
		stepRepo:             stepRepo,
		eventRepo:            eventRepo,
		timelineRepo:         timelineRepo,
		channelRepo:          channelRepo,
		userRepo:             userRepo,
		notifySvc:            notifySvc,
		userNotifyConfigRepo: userNotifyConfigRepo,
		teamRepo:             teamRepo,
		onCallShiftRepo:      onCallShiftRepo,
		logger:               logger,
		interval:             60 * time.Second,
		stopCh:               make(chan struct{}),
	}
}

// SetInterval overrides the default 60-second check interval.
func (e *EscalationExecutor) SetInterval(d time.Duration) {
	e.interval = d
}

// Start runs the escalation check loop in a background goroutine.
func (e *EscalationExecutor) Start() {
	go func() {
		ticker := time.NewTicker(e.interval)
		defer ticker.Stop()
		e.logger.Info("escalation executor started", zap.Duration("interval", e.interval))
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
				e.runOnce(ctx)
				cancel()
			case <-e.stopCh:
				e.logger.Info("escalation executor stopped")
				return
			}
		}
	}()
}

// Stop signals the background goroutine to exit.
func (e *EscalationExecutor) Stop() {
	e.once.Do(func() {
		select {
		case <-e.stopCh:
		default:
			close(e.stopCh)
		}
	})
}

// runOnce performs a single escalation check pass.
func (e *EscalationExecutor) runOnce(ctx context.Context) {
	// Fetch all currently active (firing or acknowledged) events — use a large page.
	events, _, err := e.eventRepo.List(ctx, "", "", 1, 10000)
	if err != nil {
		e.logger.Error("escalation: failed to list events", zap.Error(err))
		return
	}

	now := time.Now()
	for i := range events {
		ev := &events[i]
		// Only escalate firing events that haven't been resolved/closed/silenced.
		switch ev.Status {
		case model.EventStatusFiring:
			// OK — escalate
		default:
			continue
		}

		e.escalateEvent(ctx, ev, now)
	}
}

// escalateEvent evaluates all escalation policies and executes any due steps for the given event.
func (e *EscalationExecutor) escalateEvent(ctx context.Context, event *model.AlertEvent, now time.Time) {
	// Determine which escalation steps have already been executed by inspecting the timeline.
	executedSteps := e.executedStepOrders(ctx, event.ID)

	// Collect all enabled policies across all teams and find matching steps.
	// In a full implementation we would match policies to the event's team/labels.
	// For now we evaluate all enabled policies.
	policies, err := e.listAllEnabledPolicies(ctx)
	if err != nil {
		e.logger.Warn("escalation: failed to list policies", zap.Error(err))
		return
	}

	for _, policy := range policies {
		steps, err := e.stepRepo.ListByPolicyID(ctx, policy.ID)
		if err != nil {
			e.logger.Warn("escalation: failed to list steps",
				zap.Uint("policy_id", policy.ID), zap.Error(err))
			continue
		}

		// Sort by step order to execute in sequence.
		sort.Slice(steps, func(i, j int) bool {
			return steps[i].StepOrder < steps[j].StepOrder
		})

		for _, step := range steps {
			stepKey := fmt.Sprintf("escalation policy '%s' step %d triggered (delay: %dm)",
				policy.Name, step.StepOrder, step.DelayMinutes)
			if executedSteps[stepKey] {
				// Already executed this step for this event.
				continue
			}

			// Check if enough time has passed since the alert fired.
			dueAt := event.FiredAt.Add(time.Duration(step.DelayMinutes) * time.Minute)
			if now.Before(dueAt) {
				// Not due yet; later steps will be even less due.
				break
			}

			// Execute this step.
			if err := e.executeStep(ctx, event, &policy, &step); err != nil {
				e.logger.Error("escalation: failed to execute step",
					zap.Uint("event_id", event.ID),
					zap.Uint("policy_id", policy.ID),
					zap.Int("step_order", step.StepOrder),
					zap.Error(err),
				)
				// Record failure in timeline so we don't retry endlessly this cycle.
				e.recordTimeline(ctx, event.ID, fmt.Sprintf(
					"escalation step %d (policy %s) failed: %v", step.StepOrder, policy.Name, err,
				))
			}
		}
	}
}

// executeStep dispatches a notification for a single escalation step.
func (e *EscalationExecutor) executeStep(ctx context.Context, event *model.AlertEvent, policy *model.EscalationPolicy, step *model.EscalationStep) error {
	// This note is also used as the dedup key in executedStepOrders — keep format in sync.
	note := fmt.Sprintf("escalation policy '%s' step %d triggered (delay: %dm)",
		policy.Name, step.StepOrder, step.DelayMinutes)

	// Resolve the notification channel: prefer the step's override channel, then fall
	// back to notifying the target user/team directly via a system message.
	if step.NotifyChannelID != nil {
		channel, err := e.channelRepo.GetByID(ctx, *step.NotifyChannelID)
		if err != nil {
			return fmt.Errorf("channel %d not found: %w", *step.NotifyChannelID, err)
		}
		if err := e.notifySvc.SendNotification(ctx, event, channel, nil, nil); err != nil {
			return fmt.Errorf("send notification via channel %d: %w", *step.NotifyChannelID, err)
		}
	} else {
		// No channel override — dispatch directly to the target via personal notify configs.
		if err := e.dispatchToTarget(ctx, event, step); err != nil {
			return fmt.Errorf("dispatch to target %s/%d: %w", step.TargetType, step.TargetID, err)
		}
	}

	// Record the escalation in the timeline so we don't repeat this step.
	e.recordTimeline(ctx, event.ID, note)

	e.logger.Info("escalation step executed",
		zap.Uint("event_id", event.ID),
		zap.String("policy", policy.Name),
		zap.Int("step_order", step.StepOrder),
	)
	return nil
}

// executedStepOrders returns a set of "policyID:stepOrder" keys already recorded in the
// event's timeline with action=escalated.
func (e *EscalationExecutor) executedStepOrders(ctx context.Context, eventID uint) map[string]bool {
	timelines, err := e.timelineRepo.ListByEventID(ctx, eventID)
	if err != nil {
		return map[string]bool{}
	}
	result := make(map[string]bool)
	for _, t := range timelines {
		if t.Action == model.TimelineActionEscalated {
			// The note encodes the step identity — extract the key from the note prefix.
			// We use the note text as the de-dup key directly.
			result[t.Note] = true
		}
	}
	return result
}

// recordTimeline appends an escalation action to the event timeline.
func (e *EscalationExecutor) recordTimeline(ctx context.Context, eventID uint, note string) {
	t := &model.AlertTimeline{
		EventID: eventID,
		Action:  model.TimelineActionEscalated,
		Note:    note,
	}
	if err := e.timelineRepo.Create(ctx, t); err != nil {
		e.logger.Error("escalation: failed to record timeline",
			zap.Uint("event_id", eventID), zap.Error(err))
	}
}

// listAllEnabledPolicies returns all enabled EscalationPolicy records.
// ListByTeamID with teamID=0 skips the team filter and returns all policies.
func (e *EscalationExecutor) listAllEnabledPolicies(ctx context.Context) ([]model.EscalationPolicy, error) {
	all, err := e.policyRepo.ListByTeamID(ctx, 0)
	if err != nil {
		return nil, err
	}
	enabled := all[:0]
	for _, p := range all {
		if p.IsEnabled {
			enabled = append(enabled, p)
		}
	}
	return enabled, nil
}

// dispatchToTarget routes the escalation to the correct target based on step.TargetType.
func (e *EscalationExecutor) dispatchToTarget(ctx context.Context, event *model.AlertEvent, step *model.EscalationStep) error {
	switch step.TargetType {
	case "user":
		return e.notifyUserPersonal(ctx, event, step.TargetID)

	case "team":
		if e.teamRepo == nil {
			e.logger.Warn("escalation: teamRepo not configured, skipping team dispatch",
				zap.Uint("event_id", event.ID))
			return nil
		}
		members, err := e.teamRepo.ListMembers(ctx, step.TargetID)
		if err != nil {
			return fmt.Errorf("list team members: %w", err)
		}
		var lastErr error
		for _, m := range members {
			if err := e.notifyUserPersonal(ctx, event, m.UserID); err != nil {
				e.logger.Warn("escalation: failed to notify team member",
					zap.Uint("user_id", m.UserID), zap.Error(err))
				lastErr = err
			}
		}
		return lastErr

	case "schedule":
		if e.onCallShiftRepo == nil {
			e.logger.Warn("escalation: onCallShiftRepo not configured, skipping schedule dispatch",
				zap.Uint("event_id", event.ID))
			return nil
		}
		user, err := e.onCallShiftRepo.GetCurrentOnCallUser(ctx, step.TargetID)
		if err != nil {
			return fmt.Errorf("get current on-call user: %w", err)
		}
		if user == nil {
			e.logger.Info("escalation: no one currently on call for schedule",
				zap.Uint("schedule_id", step.TargetID))
			return nil
		}
		return e.notifyUserPersonal(ctx, event, user.ID)

	default:
		e.logger.Warn("escalation: unknown target type, skipping",
			zap.String("target_type", step.TargetType),
			zap.Uint("event_id", event.ID))
		return nil
	}
}

// notifyUserPersonal sends a personal notification to a user via their UserNotifyConfig entries.
// Supports "webhook" media type. "lark_personal" requires Bot API (future work).
func (e *EscalationExecutor) notifyUserPersonal(ctx context.Context, event *model.AlertEvent, userID uint) error {
	if e.userNotifyConfigRepo == nil {
		e.logger.Warn("escalation: userNotifyConfigRepo not configured, skipping personal notify",
			zap.Uint("user_id", userID))
		return nil
	}

	configs, err := e.userNotifyConfigRepo.ListByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list user notify configs: %w", err)
	}

	if len(configs) == 0 {
		e.logger.Info("escalation: user has no personal notify configs",
			zap.Uint("user_id", userID))
		return nil
	}

	var lastErr error
	for _, cfg := range configs {
		if !cfg.IsEnabled {
			continue
		}
		switch cfg.MediaType {
		case "webhook":
			// UserNotifyConfig webhook config: {"url": "https://..."}
			// custom_webhook channel accepts the same url field; method defaults to POST.
			syntheticChannel := &model.NotifyChannel{
				Type:   model.ChannelTypeCustom,
				Config: cfg.Config,
			}
			if err := e.notifySvc.SendNotification(ctx, event, syntheticChannel, nil, nil); err != nil {
				e.logger.Warn("escalation: personal webhook notify failed",
					zap.Uint("user_id", userID), zap.Error(err))
				lastErr = err
			}
		case "lark_personal":
			if e.larkSvc == nil {
				e.logger.Warn("escalation: larkSvc not configured, cannot send lark_personal DM",
					zap.Uint("user_id", userID))
				continue
			}
			receiveIDType, receiveID, perr := parseLarkPersonalConfig(cfg.Config)
			if perr != nil {
				e.logger.Warn("escalation: invalid lark_personal config",
					zap.Uint("user_id", userID), zap.Error(perr))
				lastErr = perr
				continue
			}
			if _, err := e.larkSvc.SendAlertCardToUser(ctx, event, nil, receiveIDType, receiveID); err != nil {
				e.logger.Warn("escalation: lark_personal DM failed",
					zap.Uint("user_id", userID),
					zap.String("receive_id_type", receiveIDType),
					zap.Error(err))
				lastErr = err
			}
		case "email":
			// UserNotifyConfig email config stores only the recipient address:
			// {"email": "user@example.com"}. The email notify channel expects full SMTP
			// credentials (host/port/user/pass). Until system-wide SMTP settings exist,
			// escalation-based personal email is routed via a shared email channel rule
			// rather than per-user config — log and skip here.
			e.logger.Info("escalation: personal email requires system-wide SMTP config (not yet available); use an email notify channel instead",
				zap.Uint("user_id", userID))
		default:
			e.logger.Warn("escalation: unsupported personal notify media type",
				zap.String("media_type", cfg.MediaType), zap.Uint("user_id", userID))
		}
	}

	return lastErr
}

// parseLarkPersonalConfig extracts the Lark DM recipient from a UserNotifyConfig
// `lark_personal` record. Accepts any of these JSON shapes (in order of preference):
//
//	{"user_id":"xxx"}       → receive_id_type=user_id
//	{"open_id":"ou_xxx"}    → receive_id_type=open_id
//	{"union_id":"on_xxx"}   → receive_id_type=union_id
//	{"lark_user_id":"xxx"}  → receive_id_type=user_id (legacy alias)
func parseLarkPersonalConfig(raw string) (receiveIDType, receiveID string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("lark_personal config is empty")
	}
	var c struct {
		UserID      string `json:"user_id"`
		OpenID      string `json:"open_id"`
		UnionID     string `json:"union_id"`
		LarkUserID  string `json:"lark_user_id"`
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return "", "", fmt.Errorf("parse lark_personal config: %w", err)
	}
	switch {
	case c.UserID != "":
		return "user_id", c.UserID, nil
	case c.LarkUserID != "":
		return "user_id", c.LarkUserID, nil
	case c.OpenID != "":
		return "open_id", c.OpenID, nil
	case c.UnionID != "":
		return "union_id", c.UnionID, nil
	default:
		return "", "", fmt.Errorf("lark_personal config missing user_id/open_id/union_id")
	}
}
