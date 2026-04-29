package engine

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/pkg/datasource"
	"github.com/sreagent/sreagent/internal/repository"
)

// AlertState tracks the state of an alert for a specific label combination.
type AlertState struct {
	Labels      map[string]string
	Status      string // "pending", "firing", "resolved"
	ActiveAt    time.Time
	FiredAt     time.Time
	ResolvedAt  time.Time
	Value       float64
	Annotations map[string]string
	// For recovery observation (留观时长)
	RecoveryHoldUntil time.Time
	// NoData tracking
	LastSeen time.Time
	// EventID of the firing event in the DB
	EventID uint
}

// RuleEvaluator evaluates a single alert rule.
type RuleEvaluator struct {
	rule        *model.AlertRule
	datasource  *model.DataSource
	states      map[string]*AlertState // key: fingerprint of label set
	db          *gorm.DB
	eventRepo   *repository.AlertEventRepository
	queryClient *datasource.QueryClient
	stateStore  StateStore // optional; nil = in-memory only
	suppressor  *LevelSuppressor
	workerPool  AlertWorkerPoolSubmiter // optional bounded goroutine pool
	onAlert     func(ctx context.Context, event *model.AlertEvent)
	mu          sync.Mutex
	stopCh      chan struct{}
	logger      *zap.Logger
}

// EngineStatus represents the status of the evaluation engine.
type EngineStatus struct {
	Running      bool   `json:"running"`
	TotalRules   int    `json:"total_rules"`
	ActiveAlerts int    `json:"active_alerts"`
	Uptime       string `json:"uptime"`
}

// Evaluator manages all rule evaluators.
type Evaluator struct {
	db           *gorm.DB
	dsRepo       *repository.DataSourceRepository
	ruleRepo     *repository.AlertRuleRepository
	eventRepo    *repository.AlertEventRepository
	timelineRepo *repository.AlertTimelineRepository
	queryClient  *datasource.QueryClient
	stateStore   StateStore              // optional; nil = in-memory only
	workerPool   AlertWorkerPoolSubmiter // optional bounded goroutine pool
	evaluators   map[uint]*RuleEvaluator // key: rule ID
	onAlert      func(ctx context.Context, event *model.AlertEvent)
	suppressor   *LevelSuppressor
	mu           sync.RWMutex
	logger       *zap.Logger
	stopCh       chan struct{}
	startedAt    time.Time
	syncInterval time.Duration
}

// AlertWorkerPoolSubmiter is the subset of AlertWorkerPool used by the evaluator.
type AlertWorkerPoolSubmiter interface {
	Submit(ctx context.Context, fn func(context.Context)) bool
	Wait()
}

// NewEvaluator creates a new alert evaluation engine.
func NewEvaluator(
	db *gorm.DB,
	dsRepo *repository.DataSourceRepository,
	ruleRepo *repository.AlertRuleRepository,
	eventRepo *repository.AlertEventRepository,
	timelineRepo *repository.AlertTimelineRepository,
	queryClient *datasource.QueryClient,
	logger *zap.Logger,
) *Evaluator {
	return &Evaluator{
		db:           db,
		dsRepo:       dsRepo,
		ruleRepo:     ruleRepo,
		eventRepo:    eventRepo,
		timelineRepo: timelineRepo,
		queryClient:  queryClient,
		evaluators:   make(map[uint]*RuleEvaluator),
		suppressor:   NewLevelSuppressor(),
		logger:       logger,
		stopCh:       make(chan struct{}),
		syncInterval: 30 * time.Second,
	}
}

// SetSyncInterval configures how often rules are synced from DB.
func (e *Evaluator) SetSyncInterval(d time.Duration) {
	if d > 0 {
		e.syncInterval = d
	}
}

// SetOnAlert sets the callback function called when a new alert event is created.
func (e *Evaluator) SetOnAlert(fn func(ctx context.Context, event *model.AlertEvent)) {
	e.onAlert = fn
}

// SetStateStore sets the optional state persistence store.
// If nil, the evaluator operates in memory-only mode.
func (e *Evaluator) SetStateStore(ss StateStore) {
	e.stateStore = ss
}

// SetWorkerPool sets the bounded goroutine pool for onAlert callbacks.
func (e *Evaluator) SetWorkerPool(p AlertWorkerPoolSubmiter) {
	e.workerPool = p
}

// Start begins the evaluation loop:
// 1. Load all enabled rules from DB
// 2. Start a goroutine for each rule
// 3. Periodically sync rules from DB (detect new/deleted/changed rules)
func (e *Evaluator) Start() {
	e.startedAt = time.Now()
	e.logger.Info("starting alert evaluator")

	// Initial sync
	e.syncRules()

	// Periodic sync loop
	go func() {
		ticker := time.NewTicker(e.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				e.syncRules()
			case <-e.stopCh:
				return
			}
		}
	}()
}

// Stop gracefully stops all evaluators.
func (e *Evaluator) Stop() {
	e.logger.Info("stopping alert evaluator")

	select {
	case <-e.stopCh:
		// Already stopped
		return
	default:
		close(e.stopCh)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for ruleID, re := range e.evaluators {
		select {
		case <-re.stopCh:
		default:
			close(re.stopCh)
		}
		delete(e.evaluators, ruleID)
	}

	e.logger.Info("alert evaluator stopped")
}

// syncRules loads rules from DB and starts/stops evaluators as needed.
func (e *Evaluator) syncRules() {
	ctx := context.Background()

	var rules []model.AlertRule
	if err := e.db.WithContext(ctx).
		Preload("DataSource").
		Where("status = ?", model.RuleStatusEnabled).
		Find(&rules).Error; err != nil {
		e.logger.Error("failed to load alert rules for sync", zap.Error(err))
		return
	}

	activeRuleIDs := make(map[uint]bool, len(rules))

	for i := range rules {
		rule := &rules[i]
		activeRuleIDs[rule.ID] = true

		e.mu.RLock()
		existing, exists := e.evaluators[rule.ID]
		e.mu.RUnlock()

		if exists {
			// Check if rule has been updated (version changed)
			existing.mu.Lock()
			oldVersion := existing.rule.Version
			existing.mu.Unlock()

			if oldVersion != rule.Version {
				e.logger.Info("rule updated, restarting evaluator",
					zap.Uint("rule_id", rule.ID),
					zap.String("name", rule.Name),
				)
				e.stopRuleEvaluator(rule.ID)
				e.startRuleEvaluators(ctx, rule)
			}
		} else {
			e.startRuleEvaluators(ctx, rule)
		}
	}

	// Stop evaluators for rules that are no longer enabled
	e.mu.RLock()
	toStop := make([]uint, 0)
	for ruleID := range e.evaluators {
		if !activeRuleIDs[ruleID] {
			toStop = append(toStop, ruleID)
		}
	}
	e.mu.RUnlock()

	for _, ruleID := range toStop {
		e.logger.Info("stopping evaluator for removed/disabled rule", zap.Uint("rule_id", ruleID))
		e.stopRuleEvaluator(ruleID)
	}

	e.logger.Debug("rule sync completed",
		zap.Int("active_rules", len(rules)),
		zap.Int("evaluators", len(e.evaluators)),
	)
}

// startRuleEvaluators dispatches rule evaluation:
// - If rule.DataSourceID is non-nil, start a single evaluator for that specific datasource.
// - If rule.DataSourceID is nil and rule.DatasourceType is set, start one evaluator per
//   enabled datasource matching that type.
func (e *Evaluator) startRuleEvaluators(ctx context.Context, rule *model.AlertRule) {
	if rule.DataSourceID != nil {
		// Specific datasource: use the preloaded DataSource (may be nil if not found)
		ds := rule.DataSource
		if ds == nil {
			e.logger.Warn("rule has datasource_id but DataSource is nil after preload — skipping",
				zap.Uint("rule_id", rule.ID))
			return
		}
		e.startRuleEvaluator(rule, ds)
		return
	}

	// No specific datasource — fan out to all enabled datasources of the declared type.
	if rule.DatasourceType == "" {
		e.logger.Warn("rule has no datasource_id and no datasource_type — skipping",
			zap.Uint("rule_id", rule.ID))
		return
	}

	dsList, err := e.dsRepo.ListEnabledByType(ctx, rule.DatasourceType)
	if err != nil {
		e.logger.Error("failed to list datasources by type for rule",
			zap.Uint("rule_id", rule.ID),
			zap.String("type", string(rule.DatasourceType)),
			zap.Error(err),
		)
		return
	}
	if len(dsList) == 0 {
		e.logger.Warn("no enabled datasources found for rule type",
			zap.Uint("rule_id", rule.ID),
			zap.String("type", string(rule.DatasourceType)),
		)
		return
	}

	// Use the first matching datasource. Future: fan-out with composite evaluator key.
	e.startRuleEvaluator(rule, &dsList[0])
}

// startRuleEvaluator creates and starts a goroutine for a single rule against a specific datasource.
func (e *Evaluator) startRuleEvaluator(rule *model.AlertRule, ds *model.DataSource) {
	re := &RuleEvaluator{
		rule:        rule,
		datasource:  ds,
		states:      make(map[string]*AlertState),
		db:          e.db,
		eventRepo:   e.eventRepo,
		queryClient: e.queryClient,
		stateStore:  e.stateStore,
		suppressor:  e.suppressor,
		workerPool:  e.workerPool,
		onAlert:     e.onAlert,
		stopCh:      make(chan struct{}),
		logger:      e.logger.With(zap.Uint("rule_id", rule.ID), zap.String("rule_name", rule.Name)),
	}

	e.mu.Lock()
	e.evaluators[rule.ID] = re
	e.mu.Unlock()

	go re.Run()

	e.logger.Info("started evaluator for rule",
		zap.Uint("rule_id", rule.ID),
		zap.String("name", rule.Name),
		zap.String("datasource", ds.Name),
	)
}

// stopRuleEvaluator stops and removes an evaluator.
func (e *Evaluator) stopRuleEvaluator(ruleID uint) {
	e.mu.Lock()
	re, exists := e.evaluators[ruleID]
	if exists {
		delete(e.evaluators, ruleID)
	}
	e.mu.Unlock()

	if exists && re != nil {
		select {
		case <-re.stopCh:
		default:
			close(re.stopCh)
		}
	}
}

// GetStatus returns status of the evaluation engine.
func (e *Evaluator) GetStatus() EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	activeAlerts := 0
	for _, re := range e.evaluators {
		re.mu.Lock()
		for _, state := range re.states {
			if state.Status == "firing" {
				activeAlerts++
			}
		}
		re.mu.Unlock()
	}

	uptime := ""
	running := false
	select {
	case <-e.stopCh:
		running = false
	default:
		running = !e.startedAt.IsZero()
	}

	if running && !e.startedAt.IsZero() {
		uptime = time.Since(e.startedAt).Truncate(time.Second).String()
	}

	return EngineStatus{
		Running:      running,
		TotalRules:   len(e.evaluators),
		ActiveAlerts: activeAlerts,
		Uptime:       uptime,
	}
}
