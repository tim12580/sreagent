package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/sreagent/sreagent/internal/model"
)

// AlertEventFilter holds the parameters for filtering alert events.
type AlertEventFilter struct {
	Status    string
	Severity  string
	ViewMode  string // "mine" | "unassigned" | "all"
	UserID    uint   // current user ID (for "mine" mode)
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

type AlertEventRepository struct {
	db *gorm.DB
}

func NewAlertEventRepository(db *gorm.DB) *AlertEventRepository {
	return &AlertEventRepository{db: db}
}

// DB returns the underlying *gorm.DB for use in custom queries.
func (r *AlertEventRepository) DB() *gorm.DB { return r.db }

func (r *AlertEventRepository) Create(ctx context.Context, event *model.AlertEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *AlertEventRepository) GetByID(ctx context.Context, id uint) (*model.AlertEvent, error) {
	var event model.AlertEvent
	err := r.db.WithContext(ctx).
		Preload("Rule").
		Preload("AckedByUser").
		Preload("AssignedUser").
		First(&event, id).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *AlertEventRepository) GetByFingerprint(ctx context.Context, fingerprint string) (*model.AlertEvent, error) {
	var event model.AlertEvent
	err := r.db.WithContext(ctx).
		Where("fingerprint = ? AND status != ?", fingerprint, model.EventStatusClosed).
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *AlertEventRepository) List(ctx context.Context, status, severity string, page, pageSize int) ([]model.AlertEvent, int64, error) {
	var list []model.AlertEvent
	var total int64

	query := r.db.WithContext(ctx).Model(&model.AlertEvent{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.
		Preload("AckedByUser").
		Preload("AssignedUser").
		Offset(offset).Limit(pageSize).
		Order("fired_at DESC").
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ListWithFilter returns alert events filtered by the given AlertEventFilter.
func (r *AlertEventRepository) ListWithFilter(ctx context.Context, filter AlertEventFilter) ([]model.AlertEvent, int64, error) {
	var list []model.AlertEvent
	var total int64

	query := r.db.WithContext(ctx).Model(&model.AlertEvent{})

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.StartTime != nil {
		query = query.Where("fired_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("fired_at <= ?", filter.EndTime)
	}

	switch filter.ViewMode {
	case "mine":
		// Also include oncall_user_id if column exists (graceful fallback)
		if filter.UserID > 0 {
			query = query.Where("assigned_to = ? OR acked_by = ?", filter.UserID, filter.UserID)
		}
	case "unassigned":
		// Use NULL check instead of is_dispatched for backward compat with old schema
		query = query.Where("assigned_to IS NULL AND acked_by IS NULL AND status = ?", model.EventStatusFiring)
		// "all" and default: no user filter
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	if err := query.
		Preload("AckedByUser").
		Preload("AssignedUser").
		Offset(offset).Limit(pageSize).
		Order("fired_at DESC").
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *AlertEventRepository) Update(ctx context.Context, event *model.AlertEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// IncrFireCount atomically increments the fire_count for a firing or acknowledged event.
// It is a targeted UPDATE that avoids a prior SELECT, used by the alert engine on every
// evaluation cycle to keep DB round-trips to a minimum.
func (r *AlertEventRepository) IncrFireCount(ctx context.Context, eventID uint) error {
	return r.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Where("id = ? AND status IN ?", eventID, []string{
			string(model.EventStatusFiring),
			string(model.EventStatusAcknowledged),
		}).
		UpdateColumn("fire_count", gorm.Expr("fire_count + 1")).
		Error
}

// BulkAcknowledge performs a single UPDATE … WHERE id IN (ids) to acknowledge firing events.
// Returns the number of rows actually updated.
func (r *AlertEventRepository) BulkAcknowledge(ctx context.Context, ids []uint, userID uint) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Where("id IN ? AND status = ?", ids, model.EventStatusFiring).
		Updates(map[string]interface{}{
			"status":     model.EventStatusAcknowledged,
			"acked_by":   userID,
			"acked_at":   now,
			"updated_at": now,
		})
	return result.RowsAffected, result.Error
}

// BulkClose closes multiple events in one UPDATE … WHERE id IN (ids).
// Returns the number of rows actually updated.
func (r *AlertEventRepository) BulkClose(ctx context.Context, ids []uint) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.AlertEvent{}).
		Where("id IN ? AND status NOT IN ?", ids, []string{
			string(model.EventStatusClosed),
			string(model.EventStatusResolved),
		}).
		Updates(map[string]interface{}{
			"status":     model.EventStatusClosed,
			"closed_at":  now,
			"updated_at": now,
		})
	return result.RowsAffected, result.Error
}

// AlertTimelineRepository handles alert timeline persistence.
type AlertTimelineRepository struct {
	db *gorm.DB
}

func NewAlertTimelineRepository(db *gorm.DB) *AlertTimelineRepository {
	return &AlertTimelineRepository{db: db}
}

func (r *AlertTimelineRepository) Create(ctx context.Context, timeline *model.AlertTimeline) error {
	return r.db.WithContext(ctx).Create(timeline).Error
}

// BulkCreate inserts multiple timeline entries in a single INSERT statement.
func (r *AlertTimelineRepository) BulkCreate(ctx context.Context, entries []model.AlertTimeline) error {
	if len(entries) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&entries).Error
}

func (r *AlertTimelineRepository) ListByEventID(ctx context.Context, eventID uint) ([]model.AlertTimeline, error) {
	var list []model.AlertTimeline
	err := r.db.WithContext(ctx).
		Preload("Operator").
		Where("event_id = ?", eventID).
		Order("created_at ASC").
		Find(&list).Error
	return list, err
}
