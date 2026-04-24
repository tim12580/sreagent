# CLAUDE.md — SREAgent AI 协作上下文

> 本文件供 Claude Code / AI Vibe Coding 工具在新会话时快速接手项目，无需重新探索代码库。
> 合并来源：原 CLAUDE.md + `.opencode/context.md`（OpenCode 生成），context.md 已删除。
> **最后更新：2026-04-24（当前 tag: v1.6.0-dev）**

---

## 项目概览

**SREAgent** — 面向 SRE/运维团队的智能告警管理平台。

| 属性 | 值 |
|------|---|
| Go Module | `github.com/sreagent/sreagent` |
| GitHub 仓库 | https://github.com/tim12580/sreagent |
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
  - 无密钥时明文存储 + WARN 日志
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
  handler/                      # Gin HTTP handler（含 oidc.go）
  service/                      # 业务逻辑层（含 oidc.go ~383 行）
  repository/                   # 数据访问层（含 alert_rule_history.go）
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
  pages/
    settings/                   # Index.vue + 6 个子组件
    schedule/                   # Index.vue + 4 个子组件
    alerts/                     # rules/ events/ history/ mute/
    notification/               # Rules、Media、Templates、Subscribe、AlertChannels
    dashboard/ datasources/
  stores/auth.ts                # Pinia auth store
  router/index.ts               # Vue Router + OIDC hash 拦截 + role guard
  i18n/                         # zh-CN.ts + en.ts
  utils/                        # alert.ts、format.ts
  styles/global.css             # CSS 变量、AI 风格基础样式
  types/index.ts                # TypeScript 接口定义（~390 行）
deploy/
  docker/Dockerfile             # 多阶段构建（Go 1.25 + Node 20 → Alpine 3.20）
  kubernetes/                   # K8s 清单（namespace/mysql/redis/app）
docs/
  architecture.md               # 架构概览
  ci-deploy.md                  # CI/CD 完整文档
  api.md                        # REST API 参考（~87 个端点）
  product-design.md             # 产品设计文档
internal/pkg/dbmigrate/migrations/   # SQL 迁移文件（当前最高：000002）
```

---

## 关键架构决策（ADR）

| ADR | 决策 | 原因 |
|-----|------|------|
| ADR-1 | AI/Lark 配置存 DB（`system_settings`），不在配置文件 | 避免密钥出现在 ConfigMap/Secret；`sync.RWMutex` + 30s TTL 内存缓存，写操作立即失效；Sentinel 模式：空字符串的敏感字段不会覆盖现有值 |
| ADR-2 | 无 docker-compose（已删除） | 部署目标是 K8s |
| ADR-3 | K8s Secret 只保留 4 项：db/redis/jwt/secret-key | AI/Lark 凭据加密存 DB |
| ADR-4 | golang-migrate 是 schema 唯一来源，GORM AutoMigrate 只作安全网 | 确保迁移可审计可回滚 |
| ADR-5 | AES-256-GCM 加密敏感字段 | 密钥不出现在 DB 明文 |
| ADR-6 | 多数据源路由：Prom/VM/VLogs/Zabbix 分发 | 支持异构监控体系；`rule_eval.go:executeQuery()` 按 `datasource.Type` 分发 |
| ADR-7 | Redis Hash 持久化引擎状态 | 服务重启后恢复飞行中告警；StateStore 接口 + StateEntry 结构体（含 Labels、Annotations、Status、时间戳、EventID）；Redis 不可用时优雅降级 |
| ADR-8 | OIDC 配置存 DB，启动时加载并与 configmap 合并 | 支持运行时配置无需重启；Authorization Code Flow → ID Token 验证 → 可配置 role claim path → 自动创建/更新用户；CSRF state cookie 验证 |
| ADR-9 | RBAC 三级权限（adminOnly/manage/operate） | 精细权限控制；`authenticated` 任何登录用户（viewer、global_viewer 可读）；Webhook 端点无认证 |

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

- **AlertEvaluator** 管理 RuleEvaluator goroutine 池（一个规则一个协程）
- **LevelSuppressor**：基于严重级别的去重
- **EscalationExecutor**：独立 goroutine，定时检查并执行升级策略
- Redis Key：`engine:state:{ruleID}` → Hash，field = fingerprint，value = JSON StateEntry
- 所有状态转换点（pending/firing/resolved/recovery_hold/nodata）均调用 persist

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

### 通知渠道配置格式

**lark_webhook / lark_bot**
```json
{"webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"}
```

**email**
```json
{
  "smtp_host": "smtp.example.com",
  "smtp_port": 587,
  "smtp_tls": true,
  "username": "alert@example.com",
  "password": "secret",
  "from": "SREAgent <alert@example.com>",
  "recipients": ["ops@example.com"]
}
```

**custom_webhook**
```json
{
  "url": "https://hooks.example.com/alert",
  "method": "POST",
  "headers": {"Authorization": "Bearer xxx"},
  "timeout_seconds": 10
}
```

---

## 认证流程

- **本地**：POST `/api/v1/auth/login` → JWT（24h）；POST `/api/v1/auth/refresh` → 7天宽限续签
- **OIDC**：`/auth/oidc/login` → Keycloak → `/auth/oidc/callback` → JWT 通过 URL fragment 传前端
- **5 角色**：`admin`、`team_lead`、`member`、`viewer`、`global_viewer`
- **默认账号**：`admin / admin123`（**首次登录必须修改密码**）

---

## 错误码约定

| 错误码 | 含义 |
|--------|------|
| `0` | 成功 |
| `10001` | 参数错误 |
| `10002` | 请求处理失败 |
| `10200` | 权限不足（RequireRole 拒绝） |
| `40001` | 未授权（JWT 验证失败） |
| `50001` | 数据库错误 |
| `50003` | 外部 API 错误 |

---

## 配置覆盖关系

```
configs/config.yaml
  ← SREAGENT_* 环境变量覆盖（28 个 Viper 绑定变量）
  ← 手动 os.Getenv：SREAGENT_SECRET_KEY / SREAGENT_DB_DEBUG / CORS_ALLOWED_ORIGINS
  ← K8s Secret 挂载为环境变量

AI / Lark 配置：
  ← AES-GCM 加密存储在 system_settings 表
  ← Web UI 设置页管理
  ← SystemSettingService.GetAIConfig / GetLarkConfig（30s 缓存）

OIDC 配置：
  ← config.yaml 的 oidc 节 / SREAGENT_OIDC_* 环境变量
  ← 字段：enabled, issuer_url, client_id, client_secret, redirect_url, scopes, role_claim
```

---

## 环境变量（必须配置）

| 变量 | 说明 |
|------|------|
| `SREAGENT_DATABASE_PASSWORD` | MySQL 密码 |
| `SREAGENT_REDIS_PASSWORD` | Redis 密码 |
| `SREAGENT_JWT_SECRET` | JWT 签名密钥（建议 32+ 字节） |
| `SREAGENT_SECRET_KEY` | AES-GCM 主密钥（64 位十六进制 = 32 字节） |

---

## CI/CD 概要

`.github/workflows/docker-build.yml`（3 个 job）：
- `test`：`go test ./...`
- `typecheck`：`npm run typecheck`
- `build-and-push`：buildx multi-arch，Push `main` → `:latest`，Tag `v*` → `:vX.Y.Z` + `:X.Y` + `:latest`，PR → `:pr-N`（仅构建）

详见 `docs/ci-deploy.md`。

---

## 数据库表结构概要

### system_settings（migration 000002）
```sql
CREATE TABLE system_settings (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  group_name VARCHAR(64) NOT NULL,
  key_name VARCHAR(64) NOT NULL,
  value TEXT NOT NULL,
  created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY idx_group_key (group_name, key_name)
);
```

**group 取值**：`ai`（provider/api_key[加密]/base_url/model/enabled）、`lark`（app_id/app_secret[加密]/default_webhook/verification_token[加密]/encrypt_key[加密]/bot_enabled）

---

## 完成阶段记录（Phase 0-8）

| 阶段 | 内容 | 状态 |
|------|------|------|
| Phase 0 | Cleanup（删除遗留文件、修复 Dockerfile/K8s 配置、清理前端） | ✅ |
| Phase 1 | CI/CD 完整文档（`docs/ci-deploy.md`） | ✅ |
| Phase 2 | Redis 引擎状态持久化（StateStore 接口 + Redis Hash 实现） | ✅ |
| Phase 3 | Keycloak OIDC + RBAC 权限（RequireRole 应用到所有路由） | ✅ |
| Phase 4 | 核心模块补完（Subscribe/Notify 管道接入、AlertRuleHistory） | ✅ |
| Phase 5 | 前端 UI 全面改版（7 个子阶段：组件提取、composable、OIDC、页面拆分、RBAC UI） | ✅ |
| Phase 6 | API 文档（`docs/api.md`，~87 个端点） | ✅ |
| Phase 7 | QA 多角色验证（14 个后端 + 16 个前端问题已修复） | ✅ |
| Phase 8 | 上下文压缩 + 文档更新 | ✅ |

---

## QA 修复汇总（Phase 7）

### 后端（14 项，关键/高/中均已修复）
- RequireRole / GetCurrentUserID 不安全类型断言 → comma-ok
- ChangePassword 修改了管理员自己的密码 → 改用 URL `:id` 参数
- OIDC callback 无 CSRF state 验证 → 添加 state cookie 验证
- OIDC Secure cookie flag 硬编码 false → 从 TLS/X-Forwarded-Proto 推导
- OIDC JWT 通过 query param 传递 → 改为 URL fragment
- Redis 在 HTTP Server 之前关闭 → 调换顺序
- `zap.Fatal` 在 goroutine 中阻止优雅关闭 → `zap.Error` + `os.Exit`
- StateEntry 缺少 Annotations → 添加字段

### 前端（16 项，关键/高/中均已修复）
- OIDC token 拦截更新为 hash fragment
- fetchProfile catch-all logout → 仅 401 时 logout
- 401 拦截器改用 Vue Router + 去重
- Schedule setInterval 泄露 → 生命周期管理
- MainLayout fetchProfile 移入 onMounted
- Login.vue 支持 redirect query param
- Settings 路由添加 role guard
- XSS（v-html）→ pre + text
- i18n locale 持久化到 localStorage
- Auth store role 持久化

---

## 最近发布（Release Log）

| Tag | 主要内容 |
|-----|---------|
| **v1.6.0** | 系统级 SMTP 配置（`system_settings` group=smtp，UI settings→SMTP tab，支持 TLS/认证/测试发送）；升级策略 `email` 分支接入系统 SMTP 真实发送；JWT 7天宽限续签（`POST /auth/refresh`）+前端 Axios 401 自动刷新；头像 Go 层大小校验（≤272KB data URL）；`GET /alert-events/export` CSV 流式导出+前端导出按钮；`GET /mute-rules/preview` 命中预览+前端面板；Lark OpenID→DB User 映射（`user.lark_user_id` 字段 + `GetByLarkUserID`）；个人设置新增「飞书账号绑定」tab；数据源健康检查返回 latency/version 富结果 |
| **v1.5.0** | 升级策略 `target=user/team/schedule` 的 `lark_personal` 分支接入 Lark Bot API（DM 到 user_id/open_id/union_id）；告警 AutoResolve 时同步 PATCH Lark 卡片；`LarkBotService.SendMessage` 改为优先用 Bot API 回复到触发指令的 chatID（Webhook 为兜底）；`NotifyChannel` Bot API 类型（带 chat_id）在 TestChannel 支持真发送测试卡片；`BotClient.SendDirectMessage`/`SendText` 暴露 user_id/open_id/union_id 多种 receive_id_type |
| v1.3.1 | MTTA/MTTR 升级：P50/P95 百分位、按严重程度细分、MTTA/MTTR 每日趋势折线图；品牌 logo.svg（sider/login/favicon 统一）；个人信息头像扩展为 32 个预设 emoji + 自定义上传（≤200KB，base64 data URL）；修复顶部栏保存头像后仍显示用户名首字母的 bug；GitHub Actions 收敛为 linux/amd64 单架构 + `latest`+`v<tag>` 双标签 |
| v1.3.0 | 设计系统级视觉翻新：CSS token（品牌色阶、间距、阴影、motion/typography）+ Naive UI GlobalThemeOverrides（dark + light）+ 侧栏/顶栏/登录页玻璃态皮肤 |
| v1.2.0 | 告警规则分类 tab、仪表盘分析图表（趋势 + Top 规则）、操作审计日志、表达式实时测试 |
| v1.1.x | 告警详情页改版（严重等级横幅 + 生命周期时间线）、通知模块合并为单页 Tabs |
| v1.0.x | OIDC 配置 UI（存 DB）、K8s 清单、多数据源集成、RBAC 三级权限 |

---

## 竞品对比（v1.6.0 视角）

### 我们的差异化优势
| 维度 | SREAgent | PagerDuty | Grafana OnCall | 夜莺(n9e) |
|------|:--------:|:---------:|:--------------:|:---------:|
| 飞书深度集成（Bot DM + 卡片更新 + 指令） | ✅ 最强 | ❌ | ❌ | 部分 |
| AI 根因分析（LLM 报告嵌入卡片） | ✅ | ✅（$$） | ❌ | ❌ |
| 独立告警引擎（无 Alertmanager 依赖） | ✅ | N/A | 依赖 | 依赖 |
| 设计系统 / 暗色 UI 质量 | ✅ 最优 | 中 | 中 | 差 |
| OIDC/SSO 开箱即用 | ✅ | ✅ | ✅ | 部分 |

### 功能缺口（竞品已有、我们尚未实现）

**P0 — 竞品标配，近期实现**

| 功能 | 竞品 | 难度 | 说明 |
|------|------|:----:|------|
| **心跳监控（Heartbeat）** | OpsGenie/n9e/UptimeRobot | 中 | 周期 ping URL/TCP，超时即告警；新 rule type=heartbeat，引擎内建检测器 |
| **告警抑制规则（Inhibition）** | Alertmanager/PD/OpsGenie | 中 | 当源告警触发时条件性屏蔽目标告警（≠时间窗口 MuteRule）；场景：主机宕机屏蔽其上所有服务告警 |
| **企业微信 / 钉钉通知** | n9e 标配 | 简单 | 两个 NotifyMedia 类型，API 格式公开；国内场景刚需 |

**P1 — 高价值**

| 功能 | 竞品 | 难度 | 说明 |
|------|------|:----:|------|
| **告警自愈（Auto-Remediation）** | PD/n9e | 中 | 规则可选 `remediation_webhook`，告警触发时 POST 执行；如重启 Pod、清磁盘 |
| **iCal 值班日历导出** | PD/OpsGenie/OnCall | 简单 | `GET /schedules/:id/ical` 返回 RFC 5545，可导入 Google Calendar/Outlook |
| **告警 SLA 超时自动升级** | PD/OpsGenie | 中 | 告警 N 分钟未认领自动触发升级策略（升级策略已有，SLA 打通缺失） |
| **入站 Webhook 解析器** | PD/n9e | 较难 | 自定义 JSONPath 规则将第三方格式映射为 AlertEvent（目前仅 Alertmanager 格式） |

**P2 — 中等优先级**

| 功能 | 难度 | 说明 |
|------|:----:|------|
| **Postmortem / 事后分析** | 中 | 告警关闭后关联复盘记录（时间线、根因、修复措施）；PD/OpsGenie 标配 |
| **通知限频 per user** | 简单 | 同规则对同用户每小时最多通知 N 次；防告警疲劳 |
| **图表截图附件** | 较难 | 告警触发时截取数据源图表附在飞书卡片中 |
| **SOP 知识库** | 较难 | 规则关联 Markdown RunBook；AI 搜索推荐 |

**P3 — 长期规划**

| 功能 | 难度 | 说明 |
|------|:----:|------|
| 状态页（Status Page） | 大 | 公开/私有服务状态展示 |
| SMS / 电话通知 | 中 | Twilio/云信/阿里云 |
| 多租户隔离 | 极难 | 全表加 tenant_id，架构级重构 |
| 多副本引擎 | 大 | Redis 分布式锁支持水平扩展 |

---

## 已知限制与遗留事项

- 告警引擎为内存状态机，默认 `replicas: 1`；多副本扩展需引入 Redis 分布式锁
- Lark Bot `/ack` 若用户未绑定 Open ID，回落 systemUserID=1 作为操作人
- Webhook 入站只支持 Alertmanager/VMAlert 格式，第三方系统需适配
- `larkbot.go:SendMessage` 始终发到 DefaultWebhook，chatID 未使用（已知设计限制）
