# CLAUDE.md — SREAgent AI 协作上下文

> 本文件供 Claude Code / AI Vibe Coding 工具在新会话时快速接手项目，无需重新探索代码库。
> 同步来源：`.opencode/context.md`（OpenCode 生成）+ 代码审阅。
> **最后更新：2026-04-17（当前 tag: v1.5.0）**

---

## 项目概览

**SREAgent** — 面向 SRE/运维团队的智能告警管理平台。

| 属性 | 值 |
|------|---|
| Go Module | `github.com/sreagent/sreagent` |
| Go 版本 | 1.25.0（`go.mod`） |
| 前端 | Vue 3 + TypeScript + Naive UI + Vite 6 |
| 数据库 | MySQL 8.0（GORM v2 + golang-migrate） |
| 缓存/状态 | Redis 7（引擎状态持久化 + 节流） |
| 认证 | JWT HS256 + 可选 Keycloak OIDC（go-oidc/v3） |
| 容器 | Docker 多阶段构建，多架构（amd64/arm64） |
| 编排 | Kubernetes（`deploy/kubernetes/`） |
| CI/CD | GitHub Actions（`.github/workflows/docker-build.yml`） |

---

## 代码约定

### 后端（Go）

- **分层架构**：`handler` → `service` → `repository` → `model`，严格单向依赖
- **HTTP 框架**：Gin。Handler 函数签名 `func (h *XxxHandler) Method(c *gin.Context)`
- **Handler 基类**：`internal/handler/handler.go` — 提供 `h.success(c, data)`、`h.fail(c, code, msg)` 和 **安全** `h.GetCurrentUserID(c)` (comma-ok 断言)
- **错误码**：定义在 `internal/pkg/errors/errors.go`
  - `0` 成功 / `10001` 参数错误 / `10002` 业务错误 / `10200` 权限不足 / `40001` 未授权 / `50001` DB 错误 / `50003` 外部 API 错误
- **统一响应**：`{"code": 0, "message": "ok", "data": {}}`
- **RBAC 中间件**：`internal/middleware/auth.go`
  - `adminOnly` → `admin`
  - `manage` → `admin, team_lead`
  - `operate` → `admin, team_lead, member`
  - GET 端点只需 JWT，写操作必须 RequireRole
- **数据库迁移**：golang-migrate，文件在 `internal/pkg/dbmigrate/migrations/`
  - 命名：`{6位序号}_{描述}.{up|down}.sql`（示例：`000003_add_feature.up.sql`）
  - **单语句模式**：每个 SQL 文件只包含一条语句（golang-migrate 默认限制）
  - **禁止**在迁移文件中使用 `SET NAMES`、`SET FOREIGN_KEY_CHECKS` 等多语句头
- **敏感配置加密**：AES-256-GCM，环境变量 `SREAGENT_SECRET_KEY`（64 位十六进制）
  - 加密字段：`ai.api_key`、`lark.app_secret`、`lark.verification_token`、`lark.encrypt_key`
  - 存储格式：`enc:<base64(12字节nonce + GCM密文)>`
- **日志**：`zap`，goroutine 内部不用 `zap.Fatal`（影响优雅关闭），用 `zap.Error`
- **配置读取**：Viper，前缀 `SREAGENT_`。敏感字段（`database.password` 等）需显式 `BindEnv`，不能只依赖 `AutomaticEnv`
- **OIDC 配置**：存储在 DB（`system_settings` 表 group=`oidc`），启动时加载并与 configmap 合并

### 前端（Vue 3 / TypeScript）

- **组件组织**：`web/src/pages/` 按功能分页，`components/common/` 存共享组件
- **Composables**：`useCrudModal`、`usePaginatedList`（减少重复 CRUD 模板代码）
- **状态管理**：Pinia，`stores/auth.ts` 持久化 token/user/role 到 localStorage
- **API 层**：`api/index.ts`（全部 ~87 个端点）+ `api/request.ts`（Axios，401 拦截跳 Router，去重）
- **权限控制**：`canManage`/`canOperate` 计算属性控制按钮/菜单可见性
- **OIDC**：token 从 URL hash fragment 拦截（非 query param，防 Referer 泄露）
- **安全**：禁止 `v-html`，改用 `<pre>` + 文本内容（防 XSS）
- **i18n**：`zh-CN.ts` + `en.ts`，locale 持久化到 localStorage
- **类型定义**：集中在 `web/src/types/index.ts`（~390 行）

---

## 目录结构速查

```
cmd/server/main.go              # 入口，手动 DI wiring（~450 行）
internal/
  config/config.go              # Viper 配置 + OIDCConfig struct
  model/                        # GORM 数据模型
  handler/                      # Gin HTTP handler
  service/                      # 业务逻辑层
  repository/                   # 数据访问层（GORM）
  middleware/auth.go            # JWTAuth + RequireRole（comma-ok 安全断言）
  router/router.go              # 路由注册 + RequireRole 应用
  engine/                       # 告警评估引擎
    evaluator.go                # AlertEvaluator goroutine 池
    rule_eval.go                # 每规则状态机 + Redis 持久化
    state_store.go              # StateStore 接口 + StateEntry
    suppression.go              # LevelSuppressor（级别去重）
    escalation_executor.go      # 升级策略执行器
  pkg/
    dbmigrate/                  # golang-migrate runner + embed SQL
    datasource/                 # Prom/VM/VLogs/Zabbix 客户端
    lark/                       # 飞书 Webhook + 卡片模板
    redis/                      # Redis Client + RedisStateStore
    errors/                     # 结构化错误码
web/src/
  api/                          # Axios API 客户端（index.ts + request.ts）
  components/common/            # KVEditor、PageHeader、SeverityTag、StatusTag
  composables/                  # useCrudModal、usePaginatedList
  pages/                        # 页面组件（settings/ schedule/ alerts/ notification/ 等）
  stores/auth.ts                # Pinia auth store
  router/index.ts               # Vue Router + OIDC hash 拦截 + role guard
  i18n/                         # zh-CN.ts + en.ts
  types/index.ts                # TypeScript 接口定义
deploy/
  docker/Dockerfile             # 多阶段构建（Go 1.25 + Node 20 → Alpine 3.20）
  kubernetes/                   # K8s 清单（namespace/mysql/redis/app）
internal/pkg/dbmigrate/migrations/   # SQL 迁移文件（当前最高：000002）
```

---

## 关键架构决策（ADR）

| ADR | 决策 | 原因 |
|-----|------|------|
| ADR-1 | AI/Lark 配置存 DB（`system_settings`），不在配置文件 | 避免密钥出现在 ConfigMap/Secret |
| ADR-2 | 无 docker-compose（已删除） | 部署目标是 K8s |
| ADR-3 | K8s Secret 只保留 4 项：db/redis/jwt/secret-key | AI/Lark 凭据加密存 DB |
| ADR-4 | golang-migrate 是 schema 唯一来源，GORM AutoMigrate 只作安全网 | 确保迁移可审计可回滚 |
| ADR-5 | AES-256-GCM 加密敏感字段 | 密钥不出现在 DB 明文 |
| ADR-6 | 多数据源路由：Prom/VM/VLogs/Zabbix 分发 | 支持异构监控体系 |
| ADR-7 | Redis Hash 持久化引擎状态 | 服务重启后恢复飞行中告警 |
| ADR-8 | OIDC 配置存 DB，启动时加载并与 configmap 合并 | 支持运行时配置无需重启 |
| ADR-9 | RBAC 三级权限（adminOnly/manage/operate） | 精细权限控制 |

---

## 常用开发命令

```bash
make run           # 直接运行后端
make dev           # air 热重载（需 go install github.com/air-verse/air@latest）
make test          # 运行 Go 测试
make lint          # golangci-lint
make fmt           # gofmt
make docker-up     # 启动本地 MySQL + Redis（Docker）
make docker-down   # 停止本地依赖
make web-install   # npm install
make web-dev       # 前端开发服务器 http://localhost:3000
make web-build     # 生产构建
```

---

## 数据模型关系

```
DataSource ──1:N── AlertRule ──1:N── AlertEvent ──1:N── AlertTimeline
                      │                                    │
                      ├── AlertRuleHistory（CRUD 审计）     └── 12 种 action type
                      └── BizGroup（作用域）

Team ──1:N── TeamMember ──N:1── User（含 OIDCSubject）
Team ──1:N── Schedule ──1:N── ScheduleParticipant
                  └── ScheduleOverride / Shift

EscalationPolicy ──1:N── EscalationStep（运行时 Executor）

NotifyRule（v2）── match labels → NotifyMedia
MuteRule ── match labels → 抑制通知（接入引擎 SetOnAlert 回调）
SubscribeRule ── match labels → 额外接收人（接入 processSubscriptions）
SystemSetting ── group+key KV（AI/Lark 配置，AES-GCM 加密）
```

---

## 告警引擎状态机

```
inactive → pending（for_duration）→ firing → recovery_hold → resolved
                                        └── nodata（数据缺失）
```

Redis Key：`engine:state:{ruleID}` → Hash，field = fingerprint，value = JSON StateEntry

---

## 通知管道链路

```
Engine fires
  → SetOnAlert callback（main.go）
    → MuteRuleService.IsAlertMuted()    ← 命中则静默
    → NotificationService.RouteAlert()
      → v1 策略管道
      → processSubscriptions()
        → SubscribeRuleService.FindSubscriptions()
        → NotifyRuleService.ProcessEvent()
          → SendNotification()（lark_webhook/lark_bot/email/custom_webhook）
```

---

## 认证流程

- **本地**：POST `/api/v1/auth/login` → JWT（24h，无 refresh）
- **OIDC**：`/auth/oidc/login` → Keycloak → `/auth/oidc/callback` → JWT 通过 URL fragment 传前端
- **默认账号**：`admin / admin123`（**首次登录必须修改密码**）

---

## 环境变量（必须配置）

| 变量 | 说明 |
|------|------|
| `SREAGENT_DATABASE_PASSWORD` | MySQL 密码 |
| `SREAGENT_REDIS_PASSWORD` | Redis 密码 |
| `SREAGENT_JWT_SECRET` | JWT 签名密钥（建议 32+ 字节） |
| `SREAGENT_SECRET_KEY` | AES-GCM 主密钥（64 位十六进制 = 32 字节） |

---

## 最近发布（Release Log）

| Tag | 主要内容 |
|-----|---------|
| **v1.5.0** | 升级策略 `target=user/team/schedule` 的 `lark_personal` 分支接入 Lark Bot API（DM 到 user_id/open_id/union_id）；告警 AutoResolve 时同步 PATCH Lark 卡片；`LarkBotService.SendMessage` 改为优先用 Bot API 回复到触发指令的 chatID（Webhook 为兜底）；`NotifyChannel` Bot API 类型（带 chat_id）在 TestChannel 支持真发送测试卡片；`BotClient.SendDirectMessage`/`SendText` 暴露 user_id/open_id/union_id 多种 receive_id_type |
| v1.3.1 | MTTA/MTTR 升级：P50/P95 百分位、按严重程度细分、MTTA/MTTR 每日趋势折线图；品牌 logo.svg（sider/login/favicon 统一）；个人信息头像扩展为 32 个预设 emoji + 自定义上传（≤200KB，base64 data URL）；修复顶部栏保存头像后仍显示用户名首字母的 bug；GitHub Actions 收敛为 linux/amd64 单架构 + `latest`+`v<tag>` 双标签 |
| v1.3.0 | 设计系统级视觉翻新：CSS token（品牌色阶、间距、阴影、motion/typography）+ Naive UI GlobalThemeOverrides（dark + light）+ 侧栏/顶栏/登录页玻璃态皮肤 |
| v1.2.0 | 告警规则分类 tab、仪表盘分析图表（趋势 + Top 规则）、操作审计日志、表达式实时测试 |
| v1.1.x | 告警详情页改版（严重等级横幅 + 生命周期时间线）、通知模块合并为单页 Tabs |
| v1.0.x | OIDC 配置 UI（存 DB）、K8s 清单、多数据源集成、RBAC 三级权限 |

---

## 产品功能缺口（待开发）

| 功能 | 优先级 | 难度 | 状态/说明 |
|------|:------:|:----:|---------|
| 升级策略执行（target=user/team） | 高 | 中等 | `EscalationExecutor` 已跑，缺口在 `escalation_executor.go:194`：step 无 channel 时只打日志不发通知；需注入 `UserNotifyConfigRepo`+`ScheduleService` 补 user/team 分支 |
| Lark 卡片状态更新 | 高 | 较难 | 需 Bot API（非 Webhook）+ migration 加 `lark_message_id` 字段 + 状态变更时 PATCH `/im/v1/messages/{id}/patch`；凭据已在 DB |
| Lark Bot 指令（@机器人） | 中 | 中等 | 框架存在，指令未完整实现（ack / resolve / assign 快捷命令尚未接线） |
| 告警降噪/聚合 | 中 | 较难 | 未实现；建议按 `labels + fingerprint prefix` 做时间窗口合并 |
| 告警统计报表 | 中 | 中等 | 仅有仪表盘实时看板，未做周/月趋势导出（PDF/CSV） |
| 头像后端大小校验 | 中 | 简单 | 当前仅前端限制 200KB data URL，`auth.UpdateMe` 未在 Go 层校验 `avatar` 长度 |
| 告警静默窗口预览 | 中 | 中等 | 已有 MuteRule 规则，但无 "未来 24h 将被静默的告警" 的可视化 |
| SOP 知识库 | 低 | 较难 | 未实现 |
| 多租户隔离 | 低 | 很难 | 未实现（目前 BizGroup 只作为告警作用域标签，非硬隔离） |
| JWT refresh token | 低 | 简单 | 目前 JWT 24h 过期需重新登录，无 refresh endpoint |

---

## 已知限制

- `larkbot.go:SendMessage` 始终发到 DefaultWebhook，chatID 未使用（已知限制）
- 告警引擎为内存状态机，默认 `replicas: 1`；多副本扩展需引入 Redis 分布式锁
- 无 refresh token，JWT 24h 过期后需重新登录
