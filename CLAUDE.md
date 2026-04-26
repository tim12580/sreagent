# CLAUDE.md — SREAgent AI 协作上下文

> 本文件 < 200 行，仅包含每次会话必需的信息。深度文档见 `docs/` 和 `MODULES.md`。
> **最后更新：2026-04-26（当前 tag: v1.9.10）**

---

## 项目概览

**SREAgent** — 面向 SRE/运维团队的智能告警管理平台。

| 属性 | 值 |
|------|---|
| Go Module | `github.com/sreagent/sreagent` |
| Go 1.25 | Gin + GORM v2 + golang-migrate |
| 前端 | Vue 3 + TypeScript + Naive UI + Vite 6 |
| 数据库 | MySQL 8.0 + Redis 7 |
| 认证 | JWT HS256 + 可选 Keycloak OIDC |
| CI/CD | GitHub Actions（test → typecheck → build-and-push） |
| 部署 | Docker 多阶段构建 + Kubernetes |

---

## 代码约定

### 后端（Go）

- **分层**: `handler` → `service` → `repository` → `model`，严格单向依赖
- **Handler**: `func (h *XxxHandler) Method(c *gin.Context)`，用 `h.success(c, data)` / `h.fail(c, code, msg)`
- **GetCurrentUserID**: comma-ok 断言（`id, ok := h.GetCurrentUserID(c)`）
- **RBAC**: `adminOnly`(admin) / `manage`(admin,team_lead) / `operate`(admin,team_lead,member)
- **迁移**: golang-migrate，`{6位序号}_{描述}.{up|down}.sql`，**单语句模式**，禁止 SET NAMES 等多语句头
- **加密**: AES-256-GCM，`SREAGENT_SECRET_KEY`（64位十六进制），格式 `enc:<base64(nonce+ciphertext)>`
- **日志**: `zap`，goroutine 内用 `zap.Error` 不用 `zap.Fatal`
- **配置**: Viper，前缀 `SREAGENT_`，敏感字段需显式 `BindEnv`

### 前端（Vue 3 / TypeScript）

- **组件**: `web/src/pages/` 按功能分页，`components/common/` 共享
- **Composables**: `useCrudModal`、`usePaginatedList`
- **状态**: Pinia `stores/auth.ts`，持久化到 localStorage
- **API**: `api/index.ts`（120+ 端点）+ `api/request.ts`（Axios，401 拦截）
- **权限**: `canManage`/`canOperate` 计算属性
- **安全**: 禁止 `v-html`，用 `<pre>` + text
- **i18n**: `zh-CN.ts` + `en.ts`，locale 持久化
- **类型**: `web/src/types/index.ts`

---

## 开发命令

```bash
make run / make dev    # 运行 / 热重载
make test / make lint  # 测试 / lint
make fmt / make tidy   # 格式化 / 依赖整理
make web-dev           # 前端 dev server (localhost:3000)
make web-build         # 前端生产构建
make docker-up/down    # 本地 MySQL + Redis
```

---

## 目录结构

```
cmd/server/main.go           # 入口 + 手动 DI wiring
internal/
  model/          (22 files)  # GORM 数据模型
  handler/        (30 files)  # Gin HTTP handler
  service/        (29 files)  # 业务逻辑
  repository/     (21 files)  # 数据访问
  engine/         (6 files)   # 告警评估引擎
  middleware/     (3 files)   # JWT / CORS / Logger
  router/router.go           # 路由注册（120+ 端点）
  pkg/
    dbmigrate/                # golang-migrate + 14 对迁移文件
    datasource/               # Prom/VM/VLogs/Zabbix 客户端
    lark/                     # 飞书 Webhook + Bot API
    redis/                    # Redis Client + RedisStateStore
    errors/                   # 结构化错误码
web/src/                      # Vue 3 前端
docs/                         # 深度文档
```

---

## 错误码

| 码 | 含义 |
|----|------|
| `0` | 成功 |
| `10001` | 参数错误 |
| `10002` | 业务错误 |
| `10200` | 权限不足 |
| `40001` | 未授权 |
| `50001` | DB 错误 |
| `50003` | 外部 API 错误 |

---

## 数据模型关系

```
DataSource ─1:N─ AlertRule ─1:N─ AlertEvent ─1:N─ AlertTimeline
                      │
                      ├── AlertRuleHistory
                      └── BizGroup

Team ─1:N─ TeamMember ─N:1─ User
Team ─1:N─ Schedule ─1:N─ ScheduleParticipant / Override / Shift
EscalationPolicy ─1:N─ EscalationStep
NotifyRule ── match labels ── NotifyMedia
MuteRule / InhibitionRule / SubscribeRule ── match labels
SystemSetting ── group+key KV（AES-GCM 加密）
```

---

## 必配环境变量

| 变量 | 说明 |
|------|------|
| `SREAGENT_DATABASE_PASSWORD` | MySQL 密码 |
| `SREAGENT_REDIS_PASSWORD` | Redis 密码 |
| `SREAGENT_JWT_SECRET` | JWT 签名密钥 |
| `SREAGENT_SECRET_KEY` | AES-GCM 主密钥（64 位十六进制） |

---

## 工作流模板

### 新增功能

```
我正在开发 [模块名] 的 [功能名]。
参考 docs/[module].md 和 MODULES.md。
本次目标：[具体描述]
约束：只修改 [目录/文件]，遵循 CLAUDE.md 规范。
请先给实现方案，确认后再写代码。完成后更新 CHANGELOG.md。
```

### 修复 Bug

```
Bug: [现象]
文件: [路径]
错误: [日志]
请分析原因 → 修复方案 → 修复 → 补测试覆盖。
```

### 代码审查

```
审查 [文件路径]：
1. 安全漏洞 2. 性能问题 3. 错误处理 4. 规范一致性
按严重程度排序输出。
```

---

## 深度文档索引

| 文档 | 何时加载 |
|------|----------|
| [MODULES.md](MODULES.md) | 需要了解模块清单和状态 |
| [CHANGELOG.md](CHANGELOG.md) | 需要了解变更历史 |
| [docs/architecture.md](docs/architecture.md) | 架构设计、ADR 决策 |
| [docs/alert-engine.md](docs/alert-engine.md) | 修改告警引擎相关代码 |
| [docs/notification.md](docs/notification.md) | 修改通知管道相关代码 |
| [docs/api.md](docs/api.md) | API 端点参考 |
| [docs/ci-deploy.md](docs/ci-deploy.md) | CI/CD 和部署 |
| [docs/roadmap.md](docs/roadmap.md) | 竞品分析、功能路线图 |

> 原则：每次对话只加载 CLAUDE.md（必带），按需加载其他文档。
