# SREAgent 平台 — 架构设计

> 最后更新：2026-04-26（v1.9.10）

## 概述

SREAgent 是一个面向 SRE 运维团队的智能运维平台，提供监控数据源管理、告警规则与生命周期管理、值班排班、AI 智能分析，以及与飞书深度集成的多渠道通知能力。

## 技术栈

| 组件 | 技术 | 版本 |
|------|------|------|
| 后端 | Go | 1.25 |
| HTTP 框架 | Gin | v1.10+ |
| ORM | GORM v2 | MySQL dialect |
| 数据库迁移 | golang-migrate | Embedded SQL |
| 配置管理 | Viper | SREAGENT_ 前缀环境变量覆盖 |
| 认证 | JWT (HS256) + Keycloak OIDC（可选） | go-oidc/v3 |
| 前端 | Vue 3 (Composition API) | 3.5+ |
| UI 组件库 | Naive UI | 2.x |
| 构建工具 | Vite | 6.x |
| 状态管理 / 国际化 | Pinia + vue-i18n | — |
| 数据库 | MySQL 8.0 | — |
| 缓存 / 状态 | Redis 7 | go-redis/v8 |
| 容器 | Docker（多阶段构建） | — |
| 编排 | Kubernetes | 原生 YAML 清单 |
| CI/CD | GitHub Actions | 3 个 Job 流水线 |

## 系统架构

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

## 目录结构

```
sreagent/
├── cmd/server/main.go              # 入口文件（~450 行）— 手动 DI 组装
├── internal/
│   ├── config/config.go            # Viper 配置 + OIDCConfig 结构体
│   ├── model/                      # GORM 数据模型
│   │   ├── user.go                 # User（5 种角色、3 种用户类型、OIDCSubject）
│   │   ├── alert_rule.go           # AlertRule + AlertRuleHistory
│   │   ├── alert_event.go          # AlertEvent + AlertTimeline（12 种动作类型）
│   │   ├── datasource.go           # DataSource（4 种类型）
│   │   ├── notification.go         # NotifyRule、NotifyMedia、NotifyChannel
│   │   ├── system_setting.go       # SystemSetting（group+key 键值对）
│   │   ├── team.go                 # Team + TeamMember
│   │   ├── biz_group.go            # BizGroup + BizGroupMember
│   │   ├── schedule.go             # Schedule、Participant、Override、Shift
│   │   ├── mute_rule.go            # MuteRule
│   │   ├── inhibition_rule.go      # InhibitionRule（Alertmanager 风格）
│   │   ├── alert_channel.go        # AlertChannel（虚拟接收器）
│   │   ├── label_registry.go       # LabelRegistry（标签自动补全）
│   │   ├── subscribe_rule.go       # SubscribeRule
│   │   ├── user_notify_config.go   # UserNotifyConfig（用户级通知介质配置）
│   │   ├── webhook.go              # AlertManager webhook 格式结构体
│   │   └── rule_import.go          # Prometheus 规则导入格式
│   ├── handler/                    # Gin HTTP 处理器
│   │   ├── oidc.go                 # OIDC：LoginRedirect、Callback、CallbackJSON、Config
│   │   ├── alert_event.go          # 告警生命周期端点
│   │   ├── alert_action.go         # 无认证告警操作页面（飞书卡片）
│   │   ├── alert_rule.go
│   │   ├── ai.go
│   │   ├── larkbot.go              # 飞书事件 webhook 接收器
│   │   ├── datasource.go
│   │   ├── user.go                 # ChangePassword 使用 URL :id 参数
│   │   ├── notification.go         # NotifyRule/Media/Template/Subscribe CRUD
│   │   ├── schedule.go
│   │   ├── handler.go              # 基础处理器，提供安全的 GetCurrentUserID
│   │   └── system_setting.go
│   ├── service/                    # 业务逻辑层
│   │   ├── oidc.go                 # 完整的 OIDC 服务（~383 行）
│   │   ├── notification.go         # RouteAlert + processSubscriptions + SendNotification
│   │   ├── alert_event.go          # 完整生命周期、批量操作
│   │   ├── alert_pipeline.go       # AI 流水线编排
│   │   ├── alert_context.go        # AI 上下文构建
│   │   ├── alert_rule.go           # 规则 CRUD + recordHistory
│   │   ├── ai.go                   # LLM 集成
│   │   ├── larkbot.go              # 飞书机器人指令处理
│   │   ├── system_setting.go       # AES-GCM 加密设置 + 30 秒缓存
│   │   ├── datasource.go           # 数据源 CRUD + 健康检查
│   │   ├── mute_rule.go            # IsAlertMuted() — 接入引擎回调
│   │   ├── subscribe_rule.go       # FindSubscriptions() — 接入 RouteAlert
│   │   ├── notify_rule.go          # ProcessEvent() — 接入 processSubscriptions
│   │   ├── schedule.go             # 值班轮转 + 升级策略 CRUD
│   │   ├── auth.go                 # 登录、JWT 生成
│   │   └── user.go                 # 用户 CRUD、虚拟用户
│   ├── repository/                 # 数据访问层（GORM）
│   │   ├── alert_rule_history.go   # AlertRuleHistory CRUD
│   │   └── ...                     # 每个模型对应一个文件
│   ├── middleware/
│   │   ├── auth.go                 # JWTAuth() + RequireRole()（安全的 comma-ok 断言）
│   │   ├── cors.go
│   │   └── logger.go
│   ├── router/router.go            # 所有路由注册，应用 RequireRole（admin/manage/operate）
│   ├── pkg/
│   │   ├── dbmigrate/              # golang-migrate 运行器 + 内嵌 SQL
│   │   ├── datasource/             # 查询客户端（Prom、VM、Zabbix、VLogs）
│   │   ├── lark/                   # 飞书 webhook + 卡片模板
│   │   ├── redis/                  # Redis Client + RedisStateStore
│   │   └── errors/                 # 结构化错误码
│   └── engine/
│       ├── evaluator.go            # AlertEvaluator + RuleEvaluator 协程池
│       ├── rule_eval.go            # 单规则状态机 + 持久化调用
│       ├── state_store.go          # StateStore 接口 + StateEntry
│       ├── suppression.go          # LevelSuppressor（基于严重级别的去重）
│       ├── escalation_executor.go  # 运行时升级策略执行器
│       └── heartbeat_checker.go    # 心跳规则检测器
├── web/                            # Vue 3 前端
│   └── src/
│       ├── api/
│       │   ├── index.ts            # 全部 API 端点（~87 个），包含 OIDC
│       │   └── request.ts          # Axios 实例，401 拦截器（Vue Router + 去重）
│       ├── components/
│       │   ├── common/             # KVEditor、PageHeader、SeverityTag、StatusTag
│       │   └── index.ts            # 桶导出
│       ├── composables/            # useCrudModal、usePaginatedList
│       ├── stores/auth.ts          # Token/User/Role + canManage/canOperate + 持久化
│       ├── pages/
│       │   ├── Login.vue           # 本地登录 + OIDC SSO，支持重定向
│       │   ├── dashboard/
│       │   ├── datasources/
│       │   ├── alerts/             # rules/ events/ history/ mute/ inhibition/
│       │   ├── notification/       # Rules、Media、Templates、Subscribe、AlertChannels
│       │   ├── settings/           # Index.vue + 6 个子组件
│       │   └── schedule/           # Index.vue + 4 个子组件
│       ├── layouts/MainLayout.vue  # onMounted 中 fetchProfile，基于角色的菜单
│       ├── router/index.ts         # OIDC hash fragment 拦截 + 角色守卫
│       ├── i18n/                   # zh-CN（~760 行）+ en（~743 行），locale 持久化
│       ├── utils/                  # alert.ts、format.ts
│       ├── styles/global.css       # CSS 变量、AI 风格主题
│       └── types/index.ts          # TypeScript 接口定义（~390 行）
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile              # 多阶段构建：Go 1.25 + Node 20 → Alpine 3.20
│   │   └── entrypoint.sh           # 等待 MySQL 就绪，创建数据库，启动服务
│   └── kubernetes/
│       ├── 00-namespace.yaml
│       ├── app/                    # deployment、service、ingress、configmap、secret、hpa
│       ├── mysql/                  # StatefulSet + configmap + secret
│       └── redis/                  # StatefulSet + secret
├── configs/config.example.yaml     # 全部配置项，包含 OIDC 段
├── .github/workflows/docker-build.yml  # CI：test → typecheck → build-and-push
├── Makefile                        # 14 个 target（build、run、lint、docker-*、db-migrate 等）
├── go.mod                          # module github.com/sreagent/sreagent, go 1.25.0
├── MODULES.md                      # 模块清单及状态
├── CHANGELOG.md                    # 结构化变更日志
└── docs/
    ├── architecture.md             # 本文件
    ├── alert-engine.md             # 告警引擎状态机与组件
    ├── notification.md             # 通知管道设计
    ├── api.md                      # REST API 参考（120+ 端点）
    ├── ci-deploy.md                # CI/CD 流水线文档
    ├── roadmap.md                  # 竞品分析与功能路线图
    └── product-design.md           # 产品设计文档
```

## 告警引擎

告警引擎（`internal/engine/`）是核心评估循环：

1. **AlertEvaluator**（`evaluator.go`）管理一组 `RuleEvaluator` 协程池，每个活跃的告警规则对应一个协程。
2. **RuleEvaluator**（`rule_eval.go`）实现单规则状态机：
   - `inactive` → 查询数据源 → 阈值判断 → `pending`
   - `pending`（持续时间 = `for`）→ `firing` → 创建 `AlertEvent` + 触发通知
   - `firing` → 查询结果恢复正常 → `recovery_hold` → `resolved`
   - 查询无结果时进入 `nodata` 处理
3. **LevelSuppressor**（`suppression.go`）当同一目标已存在更高级别的告警时，抑制低级别告警。
4. **多数据源路由**（`executeQuery()`）：根据 `datasource.Type` 分发到 Prometheus/VM PromQL、Zabbix JSON-RPC 或 VictoriaLogs LogsQL。
5. **EscalationExecutor**（`escalation_executor.go`）：独立协程，周期性检查 firing 状态的告警，根据配置的策略和时间执行升级步骤。

### 状态持久化（Redis）

- **StateStore 接口**（`state_store.go`）：`SaveState`、`DeleteState`、`LoadStates`、`DeleteRuleStates`
- **StateEntry 结构体**：Fingerprint、Labels、Annotations、Status、ActiveAt、FiredAt、ResolvedAt、Value、RecoveryHoldUntil、LastSeen、EventID
- **Redis 实现**（`pkg/redis/state_store.go`）：Hash key `engine:state:{ruleID}`，field = fingerprint，value = JSON 编码的 StateEntry
- **持久化时机**：所有状态转换（pending、firing、resolved、recovery_hold、nodata）均调用 `persistState()` 或 `deletePersistedState()`
- **恢复机制**：RuleEvaluator 启动时调用 `loadPersistedState()` 恢复进行中的状态
- **优雅降级**：Redis 不可用时，引擎以内存模式运行并输出警告日志

### 通知管道

从告警触发到通知送达的完整链路：

```
Engine fires alert
  → SetOnAlert callback (main.go)
    → MuteRuleService.IsAlertMuted() — 命中则静默
    → NotificationService.RouteAlert()
      → v1 策略管道（旧版）
      → processSubscriptions()
        → SubscribeRuleService.FindSubscriptions()
        → For each match: NotifyRuleService.ProcessEvent()
          → NotifyMedia lookup → SendNotification()
            → lark_webhook / lark_bot / email / custom_webhook
```

## 认证与 RBAC

### 本地认证
- 无状态 JWT（HS256，24 小时有效期，无刷新令牌）
- bcrypt 密码哈希
- 默认种子账号：`admin / admin123`

### OIDC 认证（可选）
- 基于 `go-oidc/v3` + `golang.org/x/oauth2` 的完整 Authorization Code Flow
- 配置项：`oidc.enabled`、`oidc.issuer_url`、`oidc.client_id`、`oidc.client_secret`、`oidc.redirect_url`、`oidc.scopes`、`oidc.role_claim`
- 可配置的角色声明路径（支持嵌套 JSON 路径，如 `realm_access.roles`）
- 自动创建用户：首次 OIDC 登录时自动创建或更新用户
- 通过安全 Cookie 进行 CSRF state 验证
- Token 通过 URL fragment 传递（非 query param），防止 Referer 泄露

### RBAC 权限控制
- 5 种角色：`admin`、`team_lead`、`member`、`viewer`、`global_viewer`
- 通过 `RequireRole()` 中间件实施 3 级权限控制：
  - `adminOnly`：仅 admin
  - `manage`：admin、team_lead
  - `operate`：admin、team_lead、member
- 所有写路由均应用 RequireRole；读路由仅需 JWT 认证
- Webhook 端点（`/webhooks/alertmanager`、`/lark/event`）设计上无需认证
- 前端：`canManage`/`canOperate` 计算属性控制 UI 元素的可见性
- Role 持久化到 localStorage，用于水合前的路由守卫检查

## 配置管理

通过 `SREAGENT_` 前缀 + `AutomaticEnv()` 绑定 28 个 Viper 变量。
另有 3 个手动读取的环境变量：`SREAGENT_SECRET_KEY`、`SREAGENT_DB_DEBUG`、`CORS_ALLOWED_ORIGINS`。

AI 和飞书凭据以 AES-256-GCM 加密存储在 `system_settings` 表中，通过 Web UI 设置页管理，并在内存中缓存 30 秒。

OIDC 配置位于 config.yaml / 环境变量中（不在数据库中）。

## 通知渠道

`SendNotification()` 支持的渠道类型：
- `lark_webhook` — 飞书 Incoming Webhook（互动卡片）
- `lark_bot` — 飞书机器人 Webhook
- `email` — 支持 TLS 的 SMTP
- `custom_webhook` — 任意 HTTP 回调
- `sms` — 已定义常量但尚未实现

## API 设计

- RESTful 风格，统一 `/api/v1` 前缀
- JWT Bearer token 认证（本地签发或 OIDC 签发）
- 写端点通过 RequireRole 中间件实施 RBAC
- 响应格式：`{"code": 0, "message": "ok", "data": {}}`
- 分页参数：`?page=1&page_size=20`
- ~87 个端点，详见 `docs/api.md`
- 错误码：0（成功）、10001（参数错误）、10002（业务错误）、10200（权限不足）、40001（未授权）、50001（数据库错误）、50003（外部 API 错误）

## 数据模型

核心实体及其关系：

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

## 前端架构

### 组件组织
- **共享组件**（`components/common/`）：KVEditor、PageHeader、SeverityTag、StatusTag
- **Composables**（`composables/`）：`useCrudModal`（模态框 CRUD 模式）、`usePaginatedList`（分页请求模式）
- **大型页面拆分为子组件**：
  - Settings：6 个子组件（UserManagement、TeamManagement、VirtualUsers、BizGroupManagement、AIConfig、LarkBotConfig）
  - Schedule：4 个子组件（ScheduleSidebar、ScheduleModal、ShiftModal、ParticipantsList）

### 状态管理
- Pinia auth store：token、用户资料、角色、计算属性 `canManage`/`canOperate`
- Role 持久化到 localStorage，确保在 API 水合前即可进行路由守卫检查
- Locale 持久化到 localStorage，i18n 初始化时读取

### 安全措施
- 401 拦截器使用 Vue Router（非 `window.location`），并设置去重标志
- 页面加载时从 URL hash fragment 拦截 OIDC token
- 存在 XSS 风险的 `v-html` 已替换为 `<pre>` 文本内容
- Settings 路由通过角色检查（`admin`、`team_lead`）进行守卫
