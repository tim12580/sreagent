package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/model"
	apperr "github.com/sreagent/sreagent/internal/pkg/errors"
	"github.com/sreagent/sreagent/internal/repository"
)

// MuteRuleService handles mute rule business logic.
type MuteRuleService struct {
	repo   *repository.MuteRuleRepository
	logger *zap.Logger
}

// NewMuteRuleService creates a new MuteRuleService.
func NewMuteRuleService(repo *repository.MuteRuleRepository, logger *zap.Logger) *MuteRuleService {
	return &MuteRuleService{repo: repo, logger: logger}
}

// Create creates a new mute rule.
func (s *MuteRuleService) Create(ctx context.Context, rule *model.MuteRule) error {
	if err := s.repo.Create(ctx, rule); err != nil {
		s.logger.Error("failed to create mute rule", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// GetByID returns a mute rule by ID.
func (s *MuteRuleService) GetByID(ctx context.Context, id uint) (*model.MuteRule, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	return rule, nil
}

// List returns a paginated list of mute rules.
func (s *MuteRuleService) List(ctx context.Context, page, pageSize int) ([]model.MuteRule, int64, error) {
	return s.repo.List(ctx, page, pageSize)
}

// Update updates an existing mute rule.
func (s *MuteRuleService) Update(ctx context.Context, rule *model.MuteRule) error {
	existing, err := s.repo.GetByID(ctx, rule.ID)
	if err != nil {
		return apperr.ErrNotFound
	}

	existing.Name = rule.Name
	existing.Description = rule.Description
	existing.MatchLabels = rule.MatchLabels
	existing.Severities = rule.Severities
	existing.StartTime = rule.StartTime
	existing.EndTime = rule.EndTime
	existing.PeriodicStart = rule.PeriodicStart
	existing.PeriodicEnd = rule.PeriodicEnd
	existing.DaysOfWeek = rule.DaysOfWeek
	existing.Timezone = rule.Timezone
	existing.IsEnabled = rule.IsEnabled
	existing.RuleIDs = rule.RuleIDs

	if err := s.repo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to update mute rule", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// Delete deletes a mute rule by ID.
func (s *MuteRuleService) Delete(ctx context.Context, id uint) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return apperr.ErrNotFound
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete mute rule", zap.Error(err))
		return apperr.Wrap(apperr.ErrDatabase, err)
	}
	return nil
}

// IsAlertMuted checks whether an alert event should be muted based on active mute rules.
// It loads all enabled mute rules, checks label matching, time window, and severity filter.
// Returns true if ANY mute rule matches.
func (s *MuteRuleService) IsAlertMuted(ctx context.Context, event *model.AlertEvent) bool {
	rules, err := s.repo.FindAllEnabled(ctx)
	if err != nil {
		s.logger.Error("failed to load mute rules", zap.Error(err))
		return false
	}

	now := time.Now()

	for _, rule := range rules {
		if s.matchesRule(&rule, event, now) {
			s.logger.Info("alert muted by rule",
				zap.Uint("event_id", event.ID),
				zap.String("alert_name", event.AlertName),
				zap.Uint("mute_rule_id", rule.ID),
				zap.String("mute_rule_name", rule.Name),
			)
			return true
		}
	}

	return false
}

// MatchesRule is the exported wrapper — used by the preview handler.
func (s *MuteRuleService) MatchesRule(rule *model.MuteRule, event *model.AlertEvent, now time.Time) bool {
	return s.matchesRule(rule, event, now)
}

// matchesRule checks if a single mute rule matches an alert event.
func (s *MuteRuleService) matchesRule(rule *model.MuteRule, event *model.AlertEvent, now time.Time) bool {
	// 1. Check specific rule IDs if set
	if rule.RuleIDs != "" && event.RuleID != nil {
		ruleIDs := strings.Split(rule.RuleIDs, ",")
		matched := false
		for _, idStr := range ruleIDs {
			idStr = strings.TrimSpace(idStr)
			if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
				if uint(id) == *event.RuleID {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}

	// 2. Check label matching (alert must match ALL labels in the mute rule)
	if len(rule.MatchLabels) > 0 {
		for k, v := range rule.MatchLabels {
			if eventVal, ok := event.Labels[k]; !ok || eventVal != v {
				return false
			}
		}
	}

	// 3. Check severity filter
	if rule.Severities != "" {
		sevs := strings.Split(rule.Severities, ",")
		matched := false
		for _, sev := range sevs {
			if strings.TrimSpace(sev) == string(event.Severity) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 4. Check time window
	if !s.isInTimeWindow(rule, now) {
		return false
	}

	return true
}

// isInTimeWindow checks if the current time falls within the mute rule's time window.
func (s *MuteRuleService) isInTimeWindow(rule *model.MuteRule, now time.Time) bool {
	// Load timezone
	loc, err := time.LoadLocation(rule.Timezone)
	if err != nil {
		loc = time.Local
	}
	nowLocal := now.In(loc)

	// Check one-time window
	if rule.StartTime != nil && rule.EndTime != nil {
		if nowLocal.Before(*rule.StartTime) || nowLocal.After(*rule.EndTime) {
			return false
		}
		return true
	}

	// Check periodic window
	if rule.PeriodicStart != "" && rule.PeriodicEnd != "" {
		// Check day of week if specified
		if rule.DaysOfWeek != "" {
			weekday := int(nowLocal.Weekday())
			if weekday == 0 {
				weekday = 7 // Sunday = 7
			}
			days := strings.Split(rule.DaysOfWeek, ",")
			dayMatch := false
			for _, d := range days {
				if dayNum, err := strconv.Atoi(strings.TrimSpace(d)); err == nil {
					if dayNum == weekday {
						dayMatch = true
						break
					}
				}
			}
			if !dayMatch {
				return false
			}
		}

		// Parse periodic times
		start, errS := time.Parse("15:04", rule.PeriodicStart)
		end, errE := time.Parse("15:04", rule.PeriodicEnd)
		if errS != nil || errE != nil {
			return false
		}

		currentMinutes := nowLocal.Hour()*60 + nowLocal.Minute()
		startMinutes := start.Hour()*60 + start.Minute()
		endMinutes := end.Hour()*60 + end.Minute()

		if startMinutes <= endMinutes {
			// Normal range: e.g., 02:00 - 06:00
			return currentMinutes >= startMinutes && currentMinutes < endMinutes
		}
		// Overnight range: e.g., 22:00 - 06:00
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}

	// No time restriction - always active
	return true
}
