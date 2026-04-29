package engine

import (
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/pkg/datasource"
)

// Run is the main loop for a single rule evaluator.
func (re *RuleEvaluator) Run() {
	// Parse evaluation interval from rule (default 60s)
	interval := time.Duration(re.rule.EvalInterval) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}

	re.logger.Info("rule evaluator started",
		zap.Duration("interval", interval),
		zap.String("expression", re.rule.Expression),
	)

	// Restore persisted state from Redis (if available)
	re.loadPersistedState()

	// Run first evaluation immediately
	re.evaluate()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			re.evaluate()
		case <-re.stopCh:
			re.logger.Info("rule evaluator stopped")
			return
		}
	}
}

// evaluate performs one evaluation cycle.
func (re *RuleEvaluator) evaluate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Execute query against datasource — dispatch by datasource type
	results, err := re.executeQuery(ctx)
	if err != nil {
		re.logger.Warn("query execution failed, will retry next cycle",
			zap.Error(err),
		)
		// On error, we skip nodata detection to avoid false positives
		return
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	// Parse for_duration
	forDuration := parseDuration(re.rule.ForDuration)

	// Parse recovery_hold
	recoveryHold := parseDuration(re.rule.RecoveryHold)

	// Track which fingerprints were seen in this cycle
	seenFingerprints := make(map[string]bool, len(results))

	// 2. For each result series
	for _, result := range results {
		// Get value (use last value if multiple)
		if len(result.Values) == 0 {
			continue
		}
		value := result.Values[len(result.Values)-1].Value

		// Generate fingerprint from labels
		fp := generateFingerprint(result.Labels)
		seenFingerprints[fp] = true

		state, exists := re.states[fp]

		if !exists {
			// New alert series detected
			now := time.Now()
			state = &AlertState{
				Labels:      result.Labels,
				Value:       value,
				Annotations: map[string]string(re.rule.Annotations),
				LastSeen:    now,
			}

			if forDuration <= 0 {
				// Immediately fire — check suppression first
				severity := string(re.rule.Severity)
				if re.suppressor != nil && re.suppressor.ShouldSuppress(re.rule.ID, fp, severity) {
					re.logger.Debug("alert suppressed by higher severity",
						zap.String("fingerprint", fp),
						zap.String("severity", severity),
					)
					re.states[fp] = state
					re.persistState(fp, state)
				} else {
					state.Status = "firing"
					state.ActiveAt = now
					state.FiredAt = now
					re.states[fp] = state

					if re.suppressor != nil {
						re.suppressor.UpdateSeverity(re.rule.ID, fp, severity)
					}
					re.createAlertEvent(state, model.EventStatusFiring)
					re.persistState(fp, state)
				}
			} else {
				// Enter pending state
				state.Status = "pending"
				state.ActiveAt = now
				re.states[fp] = state
				re.persistState(fp, state)
			}
		} else {
			state.Value = value
			state.LastSeen = time.Now()
			// Reset recovery hold since alert is active again
			state.RecoveryHoldUntil = time.Time{}

			switch state.Status {
			case "pending":
				// Check if pending long enough
				if time.Since(state.ActiveAt) >= forDuration {
					now := time.Now()
					severity := string(re.rule.Severity)
					if re.suppressor != nil && re.suppressor.ShouldSuppress(re.rule.ID, fp, severity) {
						re.logger.Debug("pending alert suppressed by higher severity",
							zap.String("fingerprint", fp),
							zap.String("severity", severity),
						)
					} else {
						state.Status = "firing"
						state.FiredAt = now
						if re.suppressor != nil {
							re.suppressor.UpdateSeverity(re.rule.ID, fp, severity)
						}
						re.createAlertEvent(state, model.EventStatusFiring)
						re.persistState(fp, state)
					}
				}

			case "firing":
				// Update value, increment fire count if event exists
				re.updateFiringEvent(state)
				re.persistState(fp, state) // refresh TTL

			case "resolved":
				// Alert came back, re-activate
				now := time.Now()
				severity := string(re.rule.Severity)
				if forDuration <= 0 {
					if re.suppressor != nil && re.suppressor.ShouldSuppress(re.rule.ID, fp, severity) {
						re.logger.Debug("re-fired alert suppressed by higher severity",
							zap.String("fingerprint", fp),
							zap.String("severity", severity),
						)
					} else {
						state.Status = "firing"
						state.ActiveAt = now
						state.FiredAt = now
						if re.suppressor != nil {
							re.suppressor.UpdateSeverity(re.rule.ID, fp, severity)
						}
						re.createAlertEvent(state, model.EventStatusFiring)
						re.persistState(fp, state)
					}
				} else {
					state.Status = "pending"
					state.ActiveAt = now
					re.persistState(fp, state)
				}
			}
		}
	}

	// 3. Check for resolved alerts
	now := time.Now()
	for fp, state := range re.states {
		if seenFingerprints[fp] {
			continue
		}

		switch state.Status {
		case "pending":
			// Pending alert disappeared, just remove it
			delete(re.states, fp)
			re.deletePersistedState(fp)

		case "firing":
			if recoveryHold > 0 && state.RecoveryHoldUntil.IsZero() {
				// Start recovery observation period
				state.RecoveryHoldUntil = now.Add(recoveryHold)
				re.logger.Debug("alert entering recovery observation",
					zap.String("fingerprint", fp),
					zap.Duration("hold", recoveryHold),
				)
				re.persistState(fp, state)
			} else if recoveryHold > 0 && now.Before(state.RecoveryHoldUntil) {
				// Still in observation period, skip
				continue
			} else {
				// Resolve the alert
				state.Status = "resolved"
				state.ResolvedAt = now
				if re.suppressor != nil {
					re.suppressor.RemoveSeverity(re.rule.ID, fp, string(re.rule.Severity))
				}
				re.resolveAlertEvent(state)
				re.deletePersistedState(fp)
			}
		}
	}

	// 4. NoData detection
	if re.rule.NoDataEnabled && len(results) == 0 {
		noDataDuration := parseDuration(re.rule.NoDataDuration)
		if noDataDuration <= 0 {
			noDataDuration = 5 * time.Minute
		}

		noDataFP := fmt.Sprintf("nodata_%d", re.rule.ID)
		noDataState, exists := re.states[noDataFP]

		if !exists {
			// First time seeing no data - start tracking
			newState := &AlertState{
				Labels: map[string]string{
					"alertname":  re.rule.Name,
					"severity":   string(re.rule.Severity),
					"__nodata__": "true",
				},
				Status:      "pending",
				ActiveAt:    now,
				LastSeen:    now,
				Annotations: map[string]string{"description": "No data received for rule: " + re.rule.Name},
			}
			re.states[noDataFP] = newState
			re.persistState(noDataFP, newState)
		} else if noDataState.Status == "pending" && time.Since(noDataState.ActiveAt) >= noDataDuration {
			noDataState.Status = "firing"
			noDataState.FiredAt = now
			re.createAlertEvent(noDataState, model.EventStatusFiring)
			re.persistState(noDataFP, noDataState)
		}
	} else {
		// Data received, clear nodata state if it exists
		noDataFP := fmt.Sprintf("nodata_%d", re.rule.ID)
		if noDataState, exists := re.states[noDataFP]; exists && noDataState.Status == "firing" {
			noDataState.Status = "resolved"
			noDataState.ResolvedAt = now
			re.resolveAlertEvent(noDataState)
			delete(re.states, noDataFP)
			re.deletePersistedState(noDataFP)
		} else if exists {
			delete(re.states, noDataFP)
			re.deletePersistedState(noDataFP)
		}
	}
}

// generateFingerprint creates a unique fingerprint from label set.
func generateFingerprint(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		// Skip internal labels
		if strings.HasPrefix(k, "__") && strings.HasSuffix(k, "__") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}

	hash := md5.Sum([]byte(b.String()))
	return fmt.Sprintf("%x", hash)
}

// createAlertEvent creates a new alert event in the database.
func (re *RuleEvaluator) createAlertEvent(state *AlertState, status model.AlertEventStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fp := generateFingerprint(state.Labels)

	// Merge labels — priority (high → low):
	//   1. query result labels  (state.Labels)
	//   2. rule static labels   (re.rule.Labels)
	//   3. datasource labels    (re.datasource.Labels — e.g. biz_project, tenant, project)
	labels := make(model.JSONLabels)
	// Lowest priority: datasource static labels (biz_project, tenant, project, etc.)
	for k, v := range re.datasource.Labels {
		labels[k] = v
	}
	// Rule labels override datasource labels
	for k, v := range re.rule.Labels {
		labels[k] = v
	}
	// Query result labels have highest priority
	for k, v := range state.Labels {
		labels[k] = v
	}
	// Ensure severity and alertname are in labels
	if _, ok := labels["severity"]; !ok {
		labels["severity"] = string(re.rule.Severity)
	}
	if _, ok := labels["alertname"]; !ok {
		labels["alertname"] = re.rule.Name
	}

	annotations := make(model.JSONLabels)
	for k, v := range re.rule.Annotations {
		annotations[k] = v
	}
	for k, v := range state.Annotations {
		annotations[k] = v
	}

	ruleID := re.rule.ID
	event := &model.AlertEvent{
		Fingerprint: fp,
		RuleID:      &ruleID,
		AlertName:   re.rule.Name,
		Severity:    re.rule.Severity,
		Status:      status,
		Labels:      labels,
		Annotations: annotations,
		Source:      re.datasource.Name,
		FiredAt:     state.FiredAt,
		FireCount:   1,
	}

	if err := re.eventRepo.Create(ctx, event); err != nil {
		re.logger.Error("failed to create alert event",
			zap.String("fingerprint", fp),
			zap.Error(err),
		)
		return
	}

	state.EventID = event.ID

	re.logger.Info("alert fired",
		zap.Uint("event_id", event.ID),
		zap.String("alert_name", re.rule.Name),
		zap.String("severity", string(re.rule.Severity)),
		zap.Float64("value", state.Value),
	)

	// Call the onAlert callback to trigger notification routing
	if re.onAlert != nil {
		ev := event
		fn := func(ctx context.Context) {
			defer func() {
				if r := recover(); r != nil {
					re.logger.Error("panic in onAlert callback", zap.Any("recover", r))
				}
			}()
			re.onAlert(ctx, ev)
		}
		if re.workerPool != nil {
			if !re.workerPool.Submit(context.Background(), fn) {
				re.logger.Warn("worker pool full, onAlert deferred to next eval cycle",
					zap.Uint("event_id", ev.ID),
				)
			}
		} else {
			go fn(context.Background())
		}
	}
}

// updateFiringEvent atomically increments fire_count for a firing/acknowledged event
// using a single targeted UPDATE, avoiding a prior SELECT round-trip.
func (re *RuleEvaluator) updateFiringEvent(state *AlertState) {
	if state.EventID == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := re.eventRepo.IncrFireCount(ctx, state.EventID); err != nil {
		re.logger.Warn("failed to increment fire count",
			zap.Uint("event_id", state.EventID),
			zap.Error(err),
		)
	}
}

// resolveAlertEvent resolves an existing alert event.
func (re *RuleEvaluator) resolveAlertEvent(state *AlertState) {
	if state.EventID == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	event, err := re.eventRepo.GetByID(ctx, state.EventID)
	if err != nil {
		re.logger.Warn("failed to get event for resolution",
			zap.Uint("event_id", state.EventID),
			zap.Error(err),
		)
		return
	}

	if event.Status == model.EventStatusClosed || event.Status == model.EventStatusResolved {
		return
	}

	now := time.Now()
	event.Status = model.EventStatusResolved
	event.ResolvedAt = &now

	if err := re.eventRepo.Update(ctx, event); err != nil {
		re.logger.Error("failed to resolve alert event",
			zap.Uint("event_id", state.EventID),
			zap.Error(err),
		)
		return
	}

	re.logger.Info("alert resolved",
		zap.Uint("event_id", state.EventID),
		zap.String("alert_name", re.rule.Name),
	)

	// Notify about resolution
	if re.onAlert != nil {
		ev := event
		fn := func(ctx context.Context) {
			defer func() {
				if r := recover(); r != nil {
					re.logger.Error("panic in onAlert callback (resolve)", zap.Any("recover", r))
				}
			}()
			re.onAlert(ctx, ev)
		}
		if re.workerPool != nil {
			if !re.workerPool.Submit(context.Background(), fn) {
				re.logger.Warn("worker pool full, onAlert (resolve) deferred to next eval cycle",
					zap.Uint("event_id", ev.ID),
				)
			}
		} else {
			go fn(context.Background())
		}
	}
}

// parseDuration parses a duration string like "5m", "1h", "30s".
// Returns 0 on failure or empty string.
func parseDuration(s string) time.Duration {
	if s == "" || s == "0" || s == "0s" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// loadPersistedState restores alert states from the StateStore on startup.
// If no StateStore is configured or loading fails, this is a no-op (in-memory only).
func (re *RuleEvaluator) loadPersistedState() {
	if re.stateStore == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	entries, err := re.stateStore.LoadStates(ctx, re.rule.ID)
	if err != nil {
		re.logger.Warn("failed to load persisted states, starting fresh",
			zap.Error(err),
		)
		return
	}

	if len(entries) == 0 {
		return
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	restored := 0
	for fp, entry := range entries {
		state := fromStateEntry(entry)
		re.states[fp] = state
		restored++

		// Restore suppressor entries for firing states
		if state.Status == "firing" && re.suppressor != nil {
			re.suppressor.UpdateSeverity(re.rule.ID, fp, string(re.rule.Severity))
		}
	}

	re.logger.Info("restored persisted alert states",
		zap.Int("count", restored),
	)
}

// stateTTL returns the TTL for persisted state entries (3x eval interval).
func (re *RuleEvaluator) stateTTL() time.Duration {
	interval := time.Duration(re.rule.EvalInterval) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return 3 * interval
}

// persistState saves a state entry to the StateStore (if configured).
// Errors are logged but not propagated — Redis is best-effort.
func (re *RuleEvaluator) persistState(fp string, state *AlertState) {
	if re.stateStore == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := toStateEntry(fp, state)
	if err := re.stateStore.SaveState(ctx, re.rule.ID, fp, entry, re.stateTTL()); err != nil {
		re.logger.Warn("failed to persist state to redis",
			zap.String("fingerprint", fp),
			zap.Error(err),
		)
	}
}

// deletePersistedState removes a state entry from the StateStore (if configured).
func (re *RuleEvaluator) deletePersistedState(fp string) {
	if re.stateStore == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := re.stateStore.DeleteState(ctx, re.rule.ID, fp); err != nil {
		re.logger.Warn("failed to delete persisted state from redis",
			zap.String("fingerprint", fp),
			zap.Error(err),
		)
	}
}

// executeQuery dispatches the alert rule query to the appropriate backend
// based on the datasource type.
//   - Prometheus / VictoriaMetrics: PromQL instant query (/api/v1/query)
//   - Zabbix: JSON-RPC item.get by key pattern
//   - VictoriaLogs: LogsQL query (/select/logsql/query), returns match count
func (re *RuleEvaluator) executeQuery(ctx context.Context) ([]datasource.QueryResult, error) {
	ep := re.datasource.Endpoint
	at := re.datasource.AuthType
	ac := re.datasource.AuthConfig
	expr := re.rule.Expression

	switch re.datasource.Type {
	case "zabbix":
		return datasource.ZabbixInstantQuery(ctx, ep, at, ac, expr)
	case "victorialogs":
		return datasource.VictoriaLogsInstantQuery(ctx, ep, at, ac, expr)
	default:
		// prometheus, victoriametrics and any future Prometheus-compatible sources
		return re.queryClient.InstantQuery(ctx, ep, at, ac, expr, time.Time{})
	}
}
