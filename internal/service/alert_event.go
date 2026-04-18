package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/sreagent/sreagent/internal/model"
	apperr "github.com/sreagent/sreagent/internal/pkg/errors"
	"github.com/sreagent/sreagent/internal/repository"
)

// OnCallResolver is used by AlertEventService to find the current on-call person.
type OnCallResolver interface {
	GetCurrentOnCallForAlert(ctx context.Context, alertLabels map[string]string) (*model.User, error)
}

type AlertEventService struct {
	repo         *repository.AlertEventRepository
	timelineRepo *repository.AlertTimelineRepository
	notifySvc    *NotificationService
	onCallSvc    OnCallResolver
	larkSvc      *LarkService
	logger       *zap.Logger
}

// SetNotificationService wires the notification service for alert routing.
func (s *AlertEventService) SetNotificationService(svc *NotificationService) {
	s.notifySvc = svc
}

// SetOnCallResolver wires the on-call resolver for dispatch.
func (s *AlertEventService) SetOnCallResolver(r OnCallResolver) {
	s.onCallSvc = r
}

// SetLarkService wires the Lark service for in-place card updates via Bot API.
func (s *AlertEventService) SetLarkService(svc *LarkService) {
	s.larkSvc = svc
}

// DB returns the underlying database handle for advanced handler-level queries.
func (s *AlertEventService) DB() *gorm.DB { return s.repo.DB() }

func NewAlertEventService(
	repo *repository.AlertEventRepository,
	timelineRepo *repository.AlertTimelineRepository,
	logger *zap.Logger,
) *AlertEventService {
	return &AlertEventService{repo: repo, timelineRepo: timelineRepo, logger: logger}
}

func (s *AlertEventService) List(ctx context.Context, status, severity string, page, pageSize int) ([]model.AlertEvent, int64, error) {
	return s.repo.List(ctx, status, severity, page, pageSize)
}

// ListWithFilter returns alert events using the advanced filter (view mode, time range, etc.).
func (s *AlertEventService) ListWithFilter(ctx context.Context, filter repository.AlertEventFilter) ([]model.AlertEvent, int64, error) {
	return s.repo.ListWithFilter(ctx, filter)
}

func (s *AlertEventService) GetByID(ctx context.Context, id uint) (*model.AlertEvent, error) {
	event, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.ErrEventNotFound
	}
	return event, nil
}

// Acknowledge marks an alert as acknowledged.
func (s *AlertEventService) Acknowledge(ctx context.Context, eventID, userID uint) error {
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return apperr.ErrEventNotFound
	}

	if event.Status != model.EventStatusFiring {
		return apperr.WithMessage(apperr.ErrBadRequest, "alert is not in firing state")
	}

	now := time.Now()
	event.Status = model.EventStatusAcknowledged
	event.AckedBy = &userID
	event.AckedAt = &now

	if err := s.repo.Update(ctx, event); err != nil {
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	// Add timeline entry
	s.addTimeline(ctx, eventID, model.TimelineActionAcknowledged, &userID, "Alert acknowledged")

	s.triggerLarkCardUpdate(event)
	return nil
}

// Assign assigns an alert to a specific user.
func (s *AlertEventService) Assign(ctx context.Context, eventID, assignTo, operatorID uint, note string) error {
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return apperr.ErrEventNotFound
	}

	event.Status = model.EventStatusAssigned
	event.AssignedTo = &assignTo

	if err := s.repo.Update(ctx, event); err != nil {
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	if note == "" {
		note = "Alert assigned"
	}
	s.addTimeline(ctx, eventID, model.TimelineActionAssigned, &operatorID, note)

	s.triggerLarkCardUpdate(event)

	return nil
}

// Resolve marks an alert as resolved.
func (s *AlertEventService) Resolve(ctx context.Context, eventID, userID uint, resolution string) error {
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return apperr.ErrEventNotFound
	}

	now := time.Now()
	event.Status = model.EventStatusResolved
	event.ResolvedAt = &now
	event.Resolution = resolution

	if err := s.repo.Update(ctx, event); err != nil {
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	s.addTimeline(ctx, eventID, model.TimelineActionResolved, &userID, resolution)

	s.triggerLarkCardUpdate(event)
	return nil
}

// Close marks an alert as closed.
func (s *AlertEventService) Close(ctx context.Context, eventID, userID uint, note string) error {
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return apperr.ErrEventNotFound
	}

	now := time.Now()
	event.Status = model.EventStatusClosed
	event.ClosedAt = &now

	if err := s.repo.Update(ctx, event); err != nil {
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	if note == "" {
		note = "Alert closed"
	}
	s.addTimeline(ctx, eventID, model.TimelineActionClosed, &userID, note)

	s.triggerLarkCardUpdate(event)
	return nil
}

// Silence silences an alert for a specified duration.
func (s *AlertEventService) Silence(ctx context.Context, eventID, userID uint, durationMinutes int, reason string) error {
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return apperr.ErrEventNotFound
	}

	if event.Status == model.EventStatusClosed || event.Status == model.EventStatusResolved {
		return apperr.WithMessage(apperr.ErrBadRequest, "cannot silence a closed or resolved alert")
	}

	now := time.Now()
	silencedUntil := now.Add(time.Duration(durationMinutes) * time.Minute)
	event.Status = model.EventStatusSilenced
	event.SilencedUntil = &silencedUntil
	event.SilenceReason = reason

	if err := s.repo.Update(ctx, event); err != nil {
		return apperr.Wrap(apperr.ErrDatabase, err)
	}

	note := fmt.Sprintf("Alert silenced for %d minutes. Reason: %s", durationMinutes, reason)
	s.addTimeline(ctx, eventID, model.TimelineActionSilenced, &userID, note)

	s.triggerLarkCardUpdate(event)
	return nil
}

// BatchAcknowledge acknowledges multiple firing alerts in a single DB round-trip.
// Returns the number of rows updated (success) and the number of IDs that were
// not in firing state (failed = len(ids) - rows_affected).
func (s *AlertEventService) BatchAcknowledge(ctx context.Context, eventIDs []uint, userID uint) (success int, failed int, err error) {
	if len(eventIDs) == 0 {
		return 0, 0, nil
	}

	affected, dbErr := s.repo.BulkAcknowledge(ctx, eventIDs, userID)
	if dbErr != nil {
		return 0, len(eventIDs), apperr.Wrap(apperr.ErrDatabase, dbErr)
	}

	success = int(affected)
	failed = len(eventIDs) - success

	// Batch-insert timeline entries for each updated event.
	entries := make([]model.AlertTimeline, 0, success)
	for _, id := range eventIDs {
		entries = append(entries, model.AlertTimeline{
			EventID:    id,
			Action:     model.TimelineActionAcknowledged,
			OperatorID: &userID,
			Note:       "Alert acknowledged",
		})
	}
	if err2 := s.timelineRepo.BulkCreate(ctx, entries); err2 != nil {
		s.logger.Error("failed to bulk-insert acknowledge timeline", zap.Error(err2))
	}

	return success, failed, nil
}

// BatchClose closes multiple alerts in a single DB round-trip.
func (s *AlertEventService) BatchClose(ctx context.Context, eventIDs []uint, userID uint) (success int, failed int, err error) {
	if len(eventIDs) == 0 {
		return 0, 0, nil
	}

	affected, dbErr := s.repo.BulkClose(ctx, eventIDs)
	if dbErr != nil {
		return 0, len(eventIDs), apperr.Wrap(apperr.ErrDatabase, dbErr)
	}

	success = int(affected)
	failed = len(eventIDs) - success

	// Batch-insert timeline entries for each updated event.
	entries := make([]model.AlertTimeline, 0, success)
	for _, id := range eventIDs {
		entries = append(entries, model.AlertTimeline{
			EventID:    id,
			Action:     model.TimelineActionClosed,
			OperatorID: &userID,
			Note:       "Batch close",
		})
	}
	if err2 := s.timelineRepo.BulkCreate(ctx, entries); err2 != nil {
		s.logger.Error("failed to bulk-insert close timeline", zap.Error(err2))
	}

	return success, failed, nil
}

// AddComment adds a comment to the alert timeline.
func (s *AlertEventService) AddComment(ctx context.Context, eventID, userID uint, note string) error {
	if _, err := s.repo.GetByID(ctx, eventID); err != nil {
		return apperr.ErrEventNotFound
	}

	s.addTimeline(ctx, eventID, model.TimelineActionCommented, &userID, note)
	return nil
}

// GetTimeline returns the timeline for an alert event.
func (s *AlertEventService) GetTimeline(ctx context.Context, eventID uint) ([]model.AlertTimeline, error) {
	return s.timelineRepo.ListByEventID(ctx, eventID)
}

// ProcessWebhook processes an incoming AlertManager webhook payload.
func (s *AlertEventService) ProcessWebhook(ctx context.Context, payload *model.AlertManagerPayload) error {
	for _, alert := range payload.Alerts {
		if err := s.processAlert(ctx, &alert, payload); err != nil {
			s.logger.Error("failed to process alert",
				zap.String("fingerprint", alert.Fingerprint),
				zap.Error(err),
			)
			// Continue processing remaining alerts
		}
	}
	return nil
}

func (s *AlertEventService) processAlert(ctx context.Context, alert *model.AlertManagerAlert, payload *model.AlertManagerPayload) error {
	// Try to find existing event by fingerprint
	existing, _ := s.repo.GetByFingerprint(ctx, alert.Fingerprint)

	if alert.Status == "resolved" {
		if existing != nil && existing.Status != model.EventStatusClosed {
			now := time.Now()
			existing.Status = model.EventStatusResolved
			existing.ResolvedAt = &now
			if err := s.repo.Update(ctx, existing); err != nil {
				return err
			}
			s.addTimeline(ctx, existing.ID, model.TimelineActionResolved, nil, "Auto-resolved by AlertManager")
			s.triggerLarkCardUpdate(existing)
		}
		return nil
	}

	// Firing alert
	if existing != nil {
		// Dedup: increment fire count
		existing.FireCount++
		return s.repo.Update(ctx, existing)
	}

	// Determine severity from labels
	severity := model.SeverityWarning
	if sev, ok := alert.Labels["severity"]; ok {
		switch sev {
		case "critical":
			severity = model.SeverityCritical
		case "warning":
			severity = model.SeverityWarning
		case "info":
			severity = model.SeverityInfo
		}
	}

	alertName := alert.Labels["alertname"]
	if alertName == "" {
		alertName = "Unknown"
	}

	event := &model.AlertEvent{
		Fingerprint:  alert.Fingerprint,
		AlertName:    alertName,
		Severity:     severity,
		Status:       model.EventStatusFiring,
		Labels:       alert.Labels,
		Annotations:  alert.Annotations,
		Source:       payload.Receiver,
		GeneratorURL: alert.GeneratorURL,
		FiredAt:      alert.StartsAt,
		FireCount:    1,
	}

	if err := s.repo.Create(ctx, event); err != nil {
		return err
	}

	s.addTimeline(ctx, event.ID, model.TimelineActionCreated, nil, "Alert received from "+payload.Receiver)

	// On-call dispatch: find the current on-call person for matching schedules
	if s.onCallSvc != nil {
		if onCallUser, err := s.onCallSvc.GetCurrentOnCallForAlert(ctx, map[string]string(alert.Labels)); err == nil && onCallUser != nil {
			event.OnCallUserID = &onCallUser.ID
			event.IsDispatched = true
			if updateErr := s.repo.Update(ctx, event); updateErr != nil {
				s.logger.Error("failed to set on-call user on event",
					zap.Uint("event_id", event.ID),
					zap.Error(updateErr),
				)
			} else {
				note := fmt.Sprintf("Auto-dispatched to on-call user: %s", onCallUser.DisplayName)
				s.addTimeline(ctx, event.ID, model.TimelineActionDispatched, &onCallUser.ID, note)
			}
		}
	}

	// Trigger notification routing
	if s.notifySvc != nil {
		go func() {
			if err := s.notifySvc.RouteAlert(context.Background(), event); err != nil {
				s.logger.Error("failed to route alert notification",
					zap.Uint("event_id", event.ID),
					zap.Error(err),
				)
			}
		}()
	}

	s.logger.Info("new alert event created",
		zap.String("alert_name", alertName),
		zap.String("severity", string(severity)),
		zap.String("fingerprint", alert.Fingerprint),
	)

	return nil
}

// triggerLarkCardUpdate fires a background goroutine to patch the Lark card
// when the alert was originally sent via Bot API (LarkMessageID is non-empty).
func (s *AlertEventService) triggerLarkCardUpdate(event *model.AlertEvent) {
	if s.larkSvc == nil || event.LarkMessageID == "" {
		return
	}
	go func(e *model.AlertEvent) {
		if err := s.larkSvc.UpdateAlertCard(context.Background(), e, e.LarkMessageID); err != nil {
			s.logger.Warn("failed to update lark card after status change",
				zap.Uint("event_id", e.ID),
				zap.String("status", string(e.Status)),
				zap.Error(err),
			)
		}
	}(event)
}

func (s *AlertEventService) addTimeline(ctx context.Context, eventID uint, action model.AlertTimelineAction, operatorID *uint, note string) {
	timeline := &model.AlertTimeline{
		EventID:    eventID,
		Action:     action,
		OperatorID: operatorID,
		Note:       note,
	}
	if err := s.timelineRepo.Create(ctx, timeline); err != nil {
		s.logger.Error("failed to add timeline entry",
			zap.Uint("event_id", eventID),
			zap.Error(err),
		)
	}
}
