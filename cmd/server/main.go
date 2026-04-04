package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/bcrypt"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "github.com/go-sql-driver/mysql"

	"github.com/sreagent/sreagent/internal/config"
	"github.com/sreagent/sreagent/internal/engine"
	"github.com/sreagent/sreagent/internal/handler"
	"github.com/sreagent/sreagent/internal/model"
	"github.com/sreagent/sreagent/internal/pkg/datasource"
	"github.com/sreagent/sreagent/internal/pkg/dbmigrate"
	sredis "github.com/sreagent/sreagent/internal/pkg/redis"
	"github.com/sreagent/sreagent/internal/repository"
	"github.com/sreagent/sreagent/internal/router"
	"github.com/sreagent/sreagent/internal/service"
)

func main() {
	cfgFile := flag.String("config", "", "config file path")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	zapLogger := initLogger(cfg.Log)
	defer zapLogger.Sync()

	zapLogger.Info("starting SREAgent server",
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
	)

	// Initialize database
	db, err := initDatabase(cfg.Database)
	if err != nil {
		zapLogger.Fatal("failed to initialize database", zap.Error(err))
	}

	// Run database migrations (golang-migrate, version-tracked).
	// golang-migrate's MySQL driver executes the entire .sql file in a single
	// db.ExecContext call, so the connection must have multiStatements=true.
	// We open a dedicated connection for this purpose and close it immediately
	// after migrations complete; the main app connection (db) is unaffected.
	migrateDB, err := sql.Open("mysql", cfg.Database.MigrateDSN())
	if err != nil {
		zapLogger.Fatal("failed to open migration db connection", zap.Error(err))
	}
	if err := dbmigrate.RunMigrations(migrateDB, cfg.Database.Database, zapLogger); err != nil {
		_ = migrateDB.Close()
		zapLogger.Fatal("database migration failed", zap.Error(err))
	}
	_ = migrateDB.Close()

	// Auto-migrate any models not covered by SQL migrations (development safety net)
	if err := autoMigrate(db); err != nil {
		zapLogger.Fatal("failed to auto-migrate", zap.Error(err))
	}

	// Seed default admin user
	seedAdminUser(db, zapLogger)

	// Initialize repositories
	dsRepo := repository.NewDataSourceRepository(db)
	ruleRepo := repository.NewAlertRuleRepository(db)
	eventRepo := repository.NewAlertEventRepository(db)
	timelineRepo := repository.NewAlertTimelineRepository(db)
	userRepo := repository.NewUserRepository(db)
	channelRepo := repository.NewNotifyChannelRepository(db)
	policyRepo := repository.NewNotifyPolicyRepository(db)
	recordRepo := repository.NewNotifyRecordRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	participantRepo := repository.NewScheduleParticipantRepository(db)
	overrideRepo := repository.NewScheduleOverrideRepository(db)
	onCallShiftRepo := repository.NewOnCallShiftRepository(db)
	escalationPolicyRepo := repository.NewEscalationPolicyRepository(db)
	escalationStepRepo := repository.NewEscalationStepRepository(db)
	teamRepo := service.NewTeamRepository(db)
	muteRuleRepo := repository.NewMuteRuleRepository(db)
	alertRuleHistoryRepo := repository.NewAlertRuleHistoryRepository(db)

	// Phase 2 repositories
	notifyRuleRepo := repository.NewNotifyRuleRepository(db)
	notifyMediaRepo := repository.NewNotifyMediaRepository(db)
	messageTemplateRepo := repository.NewMessageTemplateRepository(db)
	subscribeRuleRepo := repository.NewSubscribeRuleRepository(db)
	bizGroupRepo := repository.NewBizGroupRepository(db)

	// Dispatch repositories
	alertChannelRepo := repository.NewAlertChannelRepository(db)
	userNotifyConfigRepo := repository.NewUserNotifyConfigRepository(db)
	systemSettingRepo := repository.NewSystemSettingRepository(db)

	// Initialize services
	settingSvc := service.NewSystemSettingService(systemSettingRepo, zapLogger)
	dsSvc := service.NewDataSourceService(dsRepo, zapLogger)
	ruleSvc := service.NewAlertRuleService(ruleRepo, alertRuleHistoryRepo, dsRepo, zapLogger)
	eventSvc := service.NewAlertEventService(eventRepo, timelineRepo, zapLogger)
	authSvc := service.NewAuthService(userRepo, &cfg.JWT, zapLogger)
	larkSvc := service.NewLarkService(zapLogger, cfg.Server.ExternalURL(), cfg.JWT.Secret)
	aiSvc := service.NewAIService(settingSvc, zapLogger)
	queryClient := datasource.NewQueryClient()
	contextBuilder := service.NewAlertContextBuilder(ruleRepo, dsRepo, queryClient, zapLogger)
	alertPipeline := service.NewAlertPipeline(contextBuilder, aiSvc, zapLogger)
	notifySvc := service.NewNotificationService(channelRepo, policyRepo, recordRepo, larkSvc, alertPipeline, zapLogger)
	userSvc := service.NewUserService(userRepo, zapLogger)
	teamSvc := service.NewTeamService(teamRepo, zapLogger)
	scheduleSvc := service.NewScheduleService(scheduleRepo, participantRepo, overrideRepo, onCallShiftRepo, escalationPolicyRepo, escalationStepRepo, zapLogger)
	muteRuleSvc := service.NewMuteRuleService(muteRuleRepo, zapLogger)

	// Phase 2 services
	notifyMediaSvc := service.NewNotifyMediaService(notifyMediaRepo, zapLogger)
	messageTemplateSvc := service.NewMessageTemplateService(messageTemplateRepo, zapLogger)
	notifyRuleSvc := service.NewNotifyRuleService(
		notifyRuleRepo, notifyMediaRepo, messageTemplateRepo, recordRepo,
		notifyMediaSvc, messageTemplateSvc, alertPipeline, zapLogger,
	)
	subscribeRuleSvc := service.NewSubscribeRuleService(subscribeRuleRepo, zapLogger)
	bizGroupSvc := service.NewBizGroupService(bizGroupRepo, zapLogger)

	// Dispatch services
	alertChannelSvc := service.NewAlertChannelService(alertChannelRepo, zapLogger)
	userNotifyConfigSvc := service.NewUserNotifyConfigService(userNotifyConfigRepo, zapLogger)

	// Seed default notification media and templates
	seedSvc := service.NewSeedService(notifyMediaRepo, messageTemplateRepo, zapLogger)
	if err := seedSvc.SeedDefaults(context.Background()); err != nil {
		zapLogger.Error("failed to seed default notification data", zap.Error(err))
	}

	larkBotSvc := service.NewLarkBotService(settingSvc, eventSvc, scheduleSvc, zapLogger)

	// Initialize OIDC service (optional).
	// Priority: DB settings (set via UI) override configmap/env values.
	// This allows admins to reconfigure OIDC without redeploying.
	// NOTE: changes to DB settings require a pod restart to take effect
	// (the OIDC provider client is initialized once at startup).
	var oidcSvc *service.OIDCService
	{
		oidcCfg := &cfg.OIDC // start with configmap/env values as baseline

		// Attempt to load from DB; merge if DB has a record.
		dbOIDC, err := settingSvc.GetOIDCConfig(context.Background())
		if err != nil {
			zapLogger.Warn("could not load OIDC config from DB, using configmap values", zap.Error(err))
		} else if dbOIDC.IssuerURL != "" || dbOIDC.Enabled {
			// DB has been configured — use DB values, falling back to configmap for any empty field.
			merged := config.OIDCConfig{
				Enabled:       dbOIDC.Enabled,
				IssuerURL:     firstNonEmpty(dbOIDC.IssuerURL, cfg.OIDC.IssuerURL),
				ClientID:      firstNonEmpty(dbOIDC.ClientID, cfg.OIDC.ClientID),
				ClientSecret:  firstNonEmpty(dbOIDC.ClientSecret, cfg.OIDC.ClientSecret),
				RedirectURL:   firstNonEmpty(dbOIDC.RedirectURL, cfg.OIDC.RedirectURL),
				RoleClaim:     firstNonEmpty(dbOIDC.RoleClaim, cfg.OIDC.RoleClaim),
				DefaultRole:   firstNonEmpty(dbOIDC.DefaultRole, cfg.OIDC.DefaultRole),
				AutoProvision: dbOIDC.AutoProvision,
			}
			// Parse scopes from DB (comma-separated string).
			if dbOIDC.Scopes != "" {
				merged.Scopes = splitScopes(dbOIDC.Scopes)
			} else {
				merged.Scopes = cfg.OIDC.Scopes
			}
			// Parse role_mapping from DB (JSON string → map).
			if dbOIDC.RoleMapping != "" {
				if rm, parseErr := parseRoleMapping(dbOIDC.RoleMapping); parseErr != nil {
					zapLogger.Warn("invalid OIDC role_mapping in DB, ignoring", zap.Error(parseErr))
					merged.RoleMapping = cfg.OIDC.RoleMapping
				} else {
					merged.RoleMapping = rm
				}
			} else {
				merged.RoleMapping = cfg.OIDC.RoleMapping
			}
			oidcCfg = &merged
			zapLogger.Info("OIDC config loaded from DB (DB values take precedence over configmap)")
		}

		if oidcCfg.Enabled {
			svc, err := service.NewOIDCService(context.Background(), oidcCfg, &cfg.JWT, userRepo, zapLogger)
			if err != nil {
				zapLogger.Error("failed to initialize OIDC service, SSO login will be unavailable", zap.Error(err))
			} else {
				oidcSvc = svc
				zapLogger.Info("OIDC service initialized",
					zap.String("issuer", oidcCfg.IssuerURL),
					zap.String("client_id", oidcCfg.ClientID),
				)
			}
		}
	}

	// Wire notification routing into alert event processing
	eventSvc.SetNotificationService(notifySvc)

	// Wire v2 subscription pipeline into notification service
	notifySvc.SetSubscribeRuleService(subscribeRuleSvc)
	notifySvc.SetNotifyRuleService(notifyRuleSvc)

	// Wire on-call resolver into alert event processing
	eventSvc.SetOnCallResolver(scheduleSvc)

	// Initialize Redis client (optional — graceful degradation if unavailable)
	var redisClient *sredis.Client
	var stateStore engine.StateStore
	if cfg.Redis.Host != "" {
		rc, err := sredis.New(&cfg.Redis)
		if err != nil {
			zapLogger.Warn("redis unavailable, engine will use in-memory state only",
				zap.String("addr", cfg.Redis.Addr()),
				zap.Error(err),
			)
		} else {
			redisClient = rc
			stateStore = sredis.NewRedisStateStore(rc, zapLogger)
			zapLogger.Info("redis connected, engine state persistence enabled",
				zap.String("addr", cfg.Redis.Addr()),
			)
		}
	} else {
		zapLogger.Info("redis not configured, engine will use in-memory state only")
	}

	// Initialize and start the escalation executor
	escalationExecutor := engine.NewEscalationExecutor(
		escalationPolicyRepo,
		escalationStepRepo,
		eventRepo,
		timelineRepo,
		channelRepo,
		userRepo,
		notifySvc,
		zapLogger,
	)
	escalationExecutor.Start()

	// Initialize alert evaluator
	var evaluator *engine.Evaluator
	var engineHandler *handler.EngineHandler

	if cfg.Engine.Enabled {
		evaluator = engine.NewEvaluator(
			db, dsRepo, ruleRepo, eventRepo, timelineRepo, queryClient, zapLogger,
		)

		// Attach optional Redis state persistence
		if stateStore != nil {
			evaluator.SetStateStore(stateStore)
		}

		// Configure sync interval
		if cfg.Engine.SyncInterval > 0 {
			evaluator.SetSyncInterval(time.Duration(cfg.Engine.SyncInterval) * time.Second)
		}

		// Set onAlert callback: check mute rules, then route notification
		evaluator.SetOnAlert(func(ctx context.Context, event *model.AlertEvent) {
			// Check mute rules first
			if muteRuleSvc.IsAlertMuted(ctx, event) {
				zapLogger.Info("alert muted, skipping notification",
					zap.Uint("event_id", event.ID),
					zap.String("alert_name", event.AlertName),
				)
				return
			}

			// Route notification
			if err := notifySvc.RouteAlert(ctx, event); err != nil {
				zapLogger.Error("failed to route alert notification",
					zap.Uint("event_id", event.ID),
					zap.Error(err),
				)
			}
		})

		// Start the evaluator
		evaluator.Start()

		engineHandler = handler.NewEngineHandler(evaluator)
	}

	// Initialize alert action handler (no-auth, token-based)
	alertActionHandler := handler.NewAlertActionHandler(eventSvc, userRepo, cfg.JWT.Secret, zapLogger)

	// Initialize handlers
	var oidcHandler *handler.OIDCHandler
	if oidcSvc != nil {
		oidcHandler = handler.NewOIDCHandler(oidcSvc)
	}

	handlers := &router.Handlers{
		Auth:             func() *handler.AuthHandler { h := handler.NewAuthHandler(authSvc); h.SetUserService(userSvc); return h }(),
		OIDC:             oidcHandler,
		OIDCSettings:     handler.NewOIDCSettingsHandler(settingSvc),
		DataSource:       handler.NewDataSourceHandler(dsSvc),
		AlertRule:        handler.NewAlertRuleHandler(ruleSvc),
		AlertEvent:       handler.NewAlertEventHandler(eventSvc),
		Notification:     handler.NewNotificationHandler(notifySvc),
		User:             handler.NewUserHandler(userSvc),
		Team:             handler.NewTeamHandler(teamSvc),
		Schedule:         handler.NewScheduleHandler(scheduleSvc),
		Dashboard:        handler.NewDashboardHandler(db, zapLogger),
		AI:               handler.NewAIHandler(aiSvc, eventSvc),
		LarkBot:          handler.NewLarkBotHandler(larkBotSvc),
		Engine:           engineHandler,
		AlertAction:      alertActionHandler,
		MuteRule:         handler.NewMuteRuleHandler(muteRuleSvc),
		NotifyRule:       handler.NewNotifyRuleHandler(notifyRuleSvc),
		NotifyMedia:      handler.NewNotifyMediaHandler(notifyMediaSvc),
		MessageTemplate:  handler.NewMessageTemplateHandler(messageTemplateSvc),
		SubscribeRule:    handler.NewSubscribeRuleHandler(subscribeRuleSvc),
		BizGroup:         handler.NewBizGroupHandler(bizGroupSvc),
		AlertChannel:     handler.NewAlertChannelHandler(alertChannelSvc),
		UserNotifyConfig: handler.NewUserNotifyConfigHandler(userNotifyConfigSvc),
	}

	// Setup router
	r := router.Setup(cfg, handlers, zapLogger)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zapLogger.Error("failed to start server", zap.Error(err))
			os.Exit(1)
		}
	}()

	zapLogger.Info("server started", zap.String("addr", cfg.Server.Addr()))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("shutting down server...")

	// Stop the escalation executor
	escalationExecutor.Stop()

	// Stop the evaluator
	if evaluator != nil {
		zapLogger.Info("stopping alert evaluator...")
		evaluator.Stop()
	}

	// Shutdown HTTP server first (drain in-flight requests)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		zapLogger.Error("server forced to shutdown", zap.Error(err))
	}

	// Close Redis connection after HTTP server has drained
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			zapLogger.Warn("failed to close redis connection", zap.Error(err))
		}
	}

	zapLogger.Info("server exited")
}

func initLogger(cfg config.LogConfig) *zap.Logger {
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	var zapCfg zap.Config
	if cfg.Format == "console" {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}
	zapCfg.Level.SetLevel(level)

	logger, _ := zapCfg.Build()
	return logger
}

func initDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	gormLogLevel := logger.Silent
	if os.Getenv("SREAGENT_DB_DEBUG") == "true" {
		gormLogLevel = logger.Info
	}

	db, err := gorm.Open(gormmysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Second)

	return db, nil
}

func autoMigrate(db *gorm.DB) error {
	// Phase 1 models
	models := []interface{}{
		&model.User{},
		&model.Team{},
		&model.DataSource{},
		&model.AlertRule{},
		&model.AlertRuleHistory{},
		&model.AlertEvent{},
		&model.AlertTimeline{},
		&model.Schedule{},
		&model.ScheduleParticipant{},
		&model.ScheduleOverride{},
		&model.OnCallShift{},
		&model.EscalationPolicy{},
		&model.EscalationStep{},
		&model.NotifyChannel{},
		&model.NotifyPolicy{},
		&model.NotifyRecord{},
		&model.MuteRule{},
	}

	// Phase 2 notification v2 models
	models = append(models, model.NotificationV2Models()...)

	// Dispatch models (alert channels + user notify configs)
	models = append(models, model.DispatchModels()...)

	// Platform settings
	models = append(models, &model.SystemSetting{})

	return db.AutoMigrate(models...)
}

func seedAdminUser(db *gorm.DB, logger *zap.Logger) {
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash password", zap.Error(err))
		return
	}

	admin := &model.User{
		Username:    "admin",
		Password:    string(hashedPwd),
		DisplayName: "Administrator",
		Email:       "admin@sreagent.local",
		Role:        model.RoleAdmin,
		IsActive:    true,
	}

	if err := db.Create(admin).Error; err != nil {
		logger.Error("failed to seed admin user", zap.Error(err))
		return
	}

	logger.Info("seeded default admin user (admin/admin123)")
}

// firstNonEmpty returns the first non-empty string from the arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// splitScopes splits a comma-separated scopes string into a slice, trimming spaces.
func splitScopes(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// parseRoleMapping parses a JSON object string into a map[string]string.
// e.g. `{"sre-admin":"admin","sre-member":"member"}` → map
func parseRoleMapping(s string) (map[string]string, error) {
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}
