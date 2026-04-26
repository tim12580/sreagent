# SREAgent Platform — Architecture

> Last updated: 2026-04-26 (v1.9.10)

## Overview

SREAgent is an intelligent SRE operations platform for managing monitoring data sources,
alert rules and lifecycle, on-call scheduling, AI-powered analysis, and multi-channel
notifications with Lark (Feishu) deep integration.

## Tech Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Backend | Go | 1.25 |
| HTTP Framework | Gin | v1.10+ |
| ORM | GORM v2 | MySQL dialect |
| Schema Migrations | golang-migrate | Embedded SQL |
| Config | Viper | SREAGENT_ prefix env override |
| Auth | JWT (HS256) + Keycloak OIDC (optional) | go-oidc/v3 |
| Frontend | Vue 3 (Composition API) | 3.5+ |
| UI Library | Naive UI | 2.x |
| Build Tool | Vite | 6.x |
| State/i18n | Pinia + vue-i18n | — |
| Database | MySQL 8.0 | — |
| Cache / State | Redis 7 | go-redis/v8 |
| Container | Docker (multi-stage) | — |
| Orchestration | Kubernetes | Plain YAML manifests |
| CI/CD | GitHub Actions | 3-job pipeline |

## System Architecture

```
                                ┌─────────────────────────────────┐
                                │         Vue 3 Frontend          │
                                │  Naive UI + TypeScript + Pinia  │
                                │  OIDC SSO + Role-based UI       │
                                └──────────────┬──────────────────┘
                                               │ HTTP REST
                                ┌──────────────▼──────────────────┐
                                │        API Layer (Gin)          │
                                │  JWT Auth / OIDC / RBAC / CORS  │
                                │  RequireRole on all write routes│
                                └──────────────┬──────────────────┘
                     ┌─────────────────────────┼─────────────────────────┐
                     │                         │                         │
          ┌──────────▼────────┐   ┌────────────▼──────────┐  ┌──────────▼────────┐
          │  DataSource Svc   │   │   Alert Engine        │  │  OnCall Svc       │
          │                   │   │                       │  │                   │
          │ - Prometheus/VM   │   │ - Rule Evaluator      │  │ - Schedule Mgmt   │
          │ - Zabbix          │   │ - State Machine       │  │ - Rotation Policy │
          │ - VictoriaLogs    │   │ - Level Suppression   │  │ - Escalation Exec │
          │ - Health Check    │   │ - Mute Rule Check     │  │ - Override        │
          └──────────┬────────┘   └────────────┬──────────┘  └──────────┬────────┘
                     │                         │                        │
          ┌──────────▼─────────────────────────▼────────────────────────▼──┐
          │                          Redis 7                               │
          │  Engine state persistence (Hash per rule)                      │
          │  Throttle / Stream helpers                                     │
          └──────────────────────────────┬─────────────────────────────────┘
                                         │
          ┌──────────────────────────────▼─────────────────────────────────┐
          │                       MySQL 8.0                                │
          │  datasources · alert_rules · alert_events · alert_timelines   │
          │  alert_rule_histories · users · teams · schedules             │
          │  notify_rules · mute_rules · subscribe_rules                  │
          │  system_settings · biz_groups · escalation_policies           │
          └────────────────────────────────────────────────────────────────┘

                     ┌───────────────────────────────────┐
                     │       External Integrations        │
                     │                                   │
                     │  ┌────────┐ ┌──────┐ ┌─────────┐ │
                     │  │ Lark   │ │ LLM  │ │  Email  │ │
                     │  │Bot+Hook│ │ API  │ │  SMTP   │ │
                     │  └────────┘ └──────┘ └─────────┘ │
                     │  ┌─────────────────┐ ┌─────────┐ │
                     │  │ Custom Webhooks │ │Keycloak │ │
                     │  └─────────────────┘ │ (OIDC)  │ │
                     │                      └─────────┘ │
                     └───────────────────────────────────┘
```

## Directory Structure

```
sreagent/
├── cmd/server/main.go              # Entry point (~450 lines) — manual DI wiring
├── internal/
│   ├── config/config.go            # Viper config + OIDCConfig struct
│   ├── model/                      # GORM models
│   │   ├── user.go                 # User (5 roles, 3 user types, OIDCSubject)
│   │   ├── alert_rule.go           # AlertRule + AlertRuleHistory
│   │   ├── alert_event.go          # AlertEvent + AlertTimeline (12 action types)
│   │   ├── datasource.go           # DataSource (4 types)
│   │   ├── notification.go         # NotifyRule, NotifyMedia, NotifyChannel
│   │   ├── system_setting.go       # SystemSetting (group+key KV)
│   │   ├── team.go                 # Team + TeamMember
│   │   ├── biz_group.go            # BizGroup + BizGroupMember
│   │   ├── schedule.go             # Schedule, Participant, Override, Shift
│   │   ├── mute_rule.go            # MuteRule
│   │   ├── inhibition_rule.go      # InhibitionRule (Alertmanager-style)
│   │   ├── alert_channel.go        # AlertChannel (virtual receiver)
│   │   ├── label_registry.go       # LabelRegistry (label autocomplete)
│   │   ├── subscribe_rule.go       # SubscribeRule
│   │   ├── user_notify_config.go   # UserNotifyConfig (per-user media)
│   │   ├── webhook.go              # AlertManager webhook format structs
│   │   └── rule_import.go          # Prometheus rule import format
│   ├── handler/                    # Gin HTTP handlers
│   │   ├── oidc.go                 # OIDC: LoginRedirect, Callback, CallbackJSON, Config
│   │   ├── alert_event.go          # Alert lifecycle endpoints
│   │   ├── alert_action.go         # No-auth alert action pages (Lark cards)
│   │   ├── alert_rule.go
│   │   ├── ai.go
│   │   ├── larkbot.go              # Lark event webhook receiver
│   │   ├── datasource.go
│   │   ├── user.go                 # ChangePassword uses URL :id param
│   │   ├── notification.go         # NotifyRule/Media/Template/Subscribe CRUD
│   │   ├── schedule.go
│   │   ├── handler.go              # Base handler with safe GetCurrentUserID
│   │   └── system_setting.go
│   ├── service/                    # Business logic
│   │   ├── oidc.go                 # Full OIDC service (~383 lines)
│   │   ├── notification.go         # RouteAlert + processSubscriptions + SendNotification
│   │   ├── alert_event.go          # Full lifecycle, batch ops
│   │   ├── alert_pipeline.go       # AI pipeline orchestration
│   │   ├── alert_context.go        # Context building for AI
│   │   ├── alert_rule.go           # Rule CRUD + recordHistory
│   │   ├── ai.go                   # LLM integration
│   │   ├── larkbot.go              # Lark bot command handling
│   │   ├── system_setting.go       # AES-GCM encrypted settings + 30s cache
│   │   ├── datasource.go           # DS CRUD + health check
│   │   ├── mute_rule.go            # IsAlertMuted() — wired into engine callback
│   │   ├── subscribe_rule.go       # FindSubscriptions() — wired into RouteAlert
│   │   ├── notify_rule.go          # ProcessEvent() — wired into processSubscriptions
│   │   ├── schedule.go             # OnCall rotation + escalation CRUD
│   │   ├── auth.go                 # Login, JWT generation
│   │   └── user.go                 # User CRUD, virtual users
│   ├── repository/                 # Data access layer (GORM)
│   │   ├── alert_rule_history.go   # AlertRuleHistory CRUD
│   │   └── ...                     # One file per model
│   ├── middleware/
│   │   ├── auth.go                 # JWTAuth() + RequireRole() (safe comma-ok assertions)
│   │   ├── cors.go
│   │   └── logger.go
│   ├── router/router.go            # All routes with RequireRole (admin/manage/operate)
│   ├── pkg/
│   │   ├── dbmigrate/              # golang-migrate runner + embedded SQL
│   │   ├── datasource/             # Query clients (Prom, VM, Zabbix, VLogs)
│   │   ├── lark/                   # Lark webhook + card templates
│   │   ├── redis/                  # Redis Client + RedisStateStore
│   │   └── errors/                 # Structured error codes
│   └── engine/
│       ├── evaluator.go            # AlertEvaluator + RuleEvaluator pool
│       ├── rule_eval.go            # Per-rule state machine + persistence calls
│       ├── state_store.go          # StateStore interface + StateEntry
│       ├── suppression.go          # LevelSuppressor (severity-based dedup)
│       ├── escalation_executor.go  # Runtime escalation executor
│       └── heartbeat_checker.go    # Heartbeat rule monitor
├── web/                            # Vue 3 frontend
│   └── src/
│       ├── api/
│       │   ├── index.ts            # All API endpoints (~87) including OIDC
│       │   └── request.ts          # Axios instance, 401 interceptor (Vue Router + dedup)
│       ├── components/
│       │   ├── common/             # KVEditor, PageHeader, SeverityTag, StatusTag
│       │   └── index.ts            # Barrel export
│       ├── composables/            # useCrudModal, usePaginatedList
│       ├── stores/auth.ts          # Token/user/role + canManage/canOperate + persistence
│       ├── pages/
│       │   ├── Login.vue           # Local + OIDC SSO, redirect support
│       │   ├── dashboard/
│       │   ├── datasources/
│       │   ├── alerts/             # rules/ events/ history/ mute/ inhibition/
│       │   ├── notification/       # Rules, Media, Templates, Subscribe, AlertChannels
│       │   ├── settings/           # Index.vue + 6 sub-components
│       │   └── schedule/           # Index.vue + 4 sub-components
│       ├── layouts/MainLayout.vue  # fetchProfile in onMounted, role-based menu
│       ├── router/index.ts         # OIDC hash fragment interception + role guard
│       ├── i18n/                   # zh-CN (~760 lines) + en (~743 lines), locale persisted
│       ├── utils/                  # alert.ts, format.ts
│       ├── styles/global.css       # CSS variables, AI-style theme
│       └── types/index.ts          # TypeScript interfaces (~390 lines)
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile              # Multi-stage: Go 1.25 + Node 20 → Alpine 3.20
│   │   └── entrypoint.sh           # Wait for MySQL, create DB, start server
│   └── kubernetes/
│       ├── 00-namespace.yaml
│       ├── app/                    # deployment, service, ingress, configmap, secret, hpa
│       ├── mysql/                  # StatefulSet + configmap + secret
│       └── redis/                  # StatefulSet + secret
├── configs/config.example.yaml     # All config keys including OIDC section
├── .github/workflows/docker-build.yml  # CI: test → typecheck → build-and-push
├── Makefile                        # 14 targets (build, run, lint, docker-*, db-migrate, etc.)
├── go.mod                          # module github.com/sreagent/sreagent, go 1.25.0
├── MODULES.md                      # Module inventory with status
├── CHANGELOG.md                    # Structured change log
└── docs/
    ├── architecture.md             # This file
    ├── alert-engine.md             # Alert engine state machine & components
    ├── notification.md             # Notification pipeline design
    ├── api.md                      # REST API reference (120+ endpoints)
    ├── ci-deploy.md                # CI/CD pipeline documentation
    ├── roadmap.md                  # Competitive analysis & feature roadmap
    └── product-design.md           # Product design document
```

## Alert Engine

The alert engine (`internal/engine/`) is the core evaluation loop:

1. **AlertEvaluator** (`evaluator.go`) manages a pool of `RuleEvaluator` goroutines, one per active alert rule.
2. **RuleEvaluator** (`rule_eval.go`) implements a per-rule state machine:
   - `inactive` → query datasource → threshold check → `pending`
   - `pending` (duration = `for`) → `firing` → create `AlertEvent` + trigger notification
   - `firing` → query returns normal → `recovery_hold` → `resolved`
   - `nodata` handling when query returns no results
3. **LevelSuppressor** (`suppression.go`) prevents lower-severity alerts when a higher-severity alert is already active for the same target.
4. **Multi-datasource routing** (`executeQuery()`): dispatches to Prometheus/VM PromQL, Zabbix JSON-RPC, or VictoriaLogs LogsQL based on `datasource.Type`.
5. **EscalationExecutor** (`escalation_executor.go`): independent goroutine that periodically checks firing alerts and executes escalation steps based on configured policies and timing.

### State Persistence (Redis)

- **StateStore interface** (`state_store.go`): `SaveState`, `DeleteState`, `LoadStates`, `DeleteRuleStates`
- **StateEntry struct**: Fingerprint, Labels, Annotations, Status, ActiveAt, FiredAt, ResolvedAt, Value, RecoveryHoldUntil, LastSeen, EventID
- **Redis implementation** (`pkg/redis/state_store.go`): Hash key `engine:state:{ruleID}`, field = fingerprint, value = JSON-encoded StateEntry
- **Persistence points**: all state transitions (pending, firing, resolved, recovery_hold, nodata) call `persistState()` or `deletePersistedState()`
- **Recovery**: `loadPersistedState()` called on RuleEvaluator startup to restore in-flight states
- **Graceful degradation**: if Redis is unavailable, engine operates in memory-only mode with warning logs

### Notification Pipeline

Complete wiring from alert to notification:

```
Engine fires alert
  → SetOnAlert callback (main.go)
    → MuteRuleService.IsAlertMuted() — suppress if matched
    → NotificationService.RouteAlert()
      → v1 policy-based pipeline (legacy)
      → processSubscriptions()
        → SubscribeRuleService.FindSubscriptions()
        → For each match: NotifyRuleService.ProcessEvent()
          → NotifyMedia lookup → SendNotification()
            → lark_webhook / lark_bot / email / custom_webhook
```

## Authentication & RBAC

### Local Authentication
- Stateless JWT (HS256, 24h expiry, no refresh token)
- bcrypt password hashing
- Default seed: `admin / admin123`

### OIDC Authentication (Optional)
- Full Authorization Code Flow via `go-oidc/v3` + `golang.org/x/oauth2`
- Config: `oidc.enabled`, `oidc.issuer_url`, `oidc.client_id`, `oidc.client_secret`, `oidc.redirect_url`, `oidc.scopes`, `oidc.role_claim`
- Configurable role claim path (supports nested JSON paths like `realm_access.roles`)
- Auto-provisioning: users created/updated on first OIDC login
- CSRF state validation via secure cookie
- Token delivery via URL fragment (not query param) to prevent Referer leakage

### RBAC Enforcement
- 5 roles: `admin`, `team_lead`, `member`, `viewer`, `global_viewer`
- 3 permission tiers applied via `RequireRole()` middleware:
  - `adminOnly`: admin
  - `manage`: admin, team_lead
  - `operate`: admin, team_lead, member
- All write routes have RequireRole applied; read routes require only JWT authentication
- Webhook endpoints (`/webhooks/alertmanager`, `/lark/event`) are unauthenticated by design
- Frontend: `canManage`/`canOperate` computed properties control UI element visibility
- Role persisted to localStorage for pre-hydration route guard checks

## Configuration

28 Viper-bound variables via `SREAGENT_` prefix + `AutomaticEnv()`.
3 manually read env vars: `SREAGENT_SECRET_KEY`, `SREAGENT_DB_DEBUG`, `CORS_ALLOWED_ORIGINS`.

AI and Lark credentials are stored encrypted (AES-256-GCM) in the `system_settings` table,
managed through the Web UI settings page, with 30-second TTL in-memory cache.

OIDC configuration is in config.yaml / environment variables (not in DB).

## Notification Channels

Supported channel types in `SendNotification()`:
- `lark_webhook` — Feishu Incoming Webhook (interactive card)
- `lark_bot` — Feishu Bot Webhook
- `email` — SMTP with TLS support
- `custom_webhook` — Arbitrary HTTP callback
- `sms` — Constant defined but not implemented

## API Design

- RESTful with `/api/v1` prefix
- JWT Bearer token authentication (local or OIDC-issued)
- RBAC via RequireRole middleware on write endpoints
- Response format: `{"code": 0, "message": "ok", "data": {}}`
- Pagination: `?page=1&page_size=20`
- ~87 endpoints documented in `docs/api.md`
- Error codes: 0 (success), 10001 (param), 10002 (business), 10200 (forbidden), 40001 (unauthorized), 50001 (DB), 50003 (external API)

## Data Model

Core entities and their relationships:

```
DataSource ──1:N── AlertRule ──1:N── AlertEvent ──1:N── AlertTimeline
                      │                                    │
                      ├── AlertRuleHistory (CRUD audit)    └── 12 action types
                      └── BizGroup (scoping)

Team ──1:N── TeamMember ──N:1── User (with OIDCSubject field)
Team ──1:N── Schedule ──1:N── ScheduleParticipant
                  └── ScheduleOverride / Shift

EscalationPolicy ──1:N── EscalationStep (with runtime Executor)

NotifyRule (v2) ── match labels → route to NotifyMedia
MuteRule ── match labels → suppress notifications (wired into engine callback)
SubscribeRule ── match labels → additional recipients (wired into processSubscriptions)

SystemSetting ── group+key KV store (AI/Lark config, AES-GCM encrypted)
```

## Frontend Architecture

### Component Organization
- **Shared components** (`components/common/`): KVEditor, PageHeader, SeverityTag, StatusTag
- **Composables** (`composables/`): `useCrudModal` (modal CRUD pattern), `usePaginatedList` (paginated fetch pattern)
- **Large pages split into sub-components**:
  - Settings: 6 sub-components (UserManagement, TeamManagement, VirtualUsers, BizGroupManagement, AIConfig, LarkBotConfig)
  - Schedule: 4 sub-components (ScheduleSidebar, ScheduleModal, ShiftModal, ParticipantsList)

### State Management
- Pinia auth store: token, user profile, role, computed `canManage`/`canOperate`
- Role persisted to localStorage for immediate route guard checks before API hydration
- Locale persisted to localStorage, read on i18n initialization

### Security
- 401 interceptor uses Vue Router (not `window.location`) with dedup flag
- OIDC token intercepted from URL hash fragment on page load
- `v-html` replaced with `<pre>` text content where XSS risk existed
- Settings route guarded by role check (`admin`, `team_lead`)
