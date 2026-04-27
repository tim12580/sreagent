# CLAUDE.md — SREAgent

> **v1.13.2** | Go 1.25 + Gin + Vue 3 + MySQL 8 + Redis 7

## 代码约定

**后端分层**: `handler` → `service` → `repository` → `model`（严格单向）

- Handler: `func (h *XxxHandler) Method(c *gin.Context)`，响应用 `Success(c, data)` / `Error(c, err)`
- GetCurrentUserID: `id, ok := h.GetCurrentUserID(c)`（comma-ok 断言）
- RBAC: `adminOnly`(admin) / `manage`(admin,team_lead) / `operate`(admin,team_lead,member)
- 迁移: `internal/pkg/dbmigrate/migrations/{序号}_{描述}.{up|down}.sql`，**单语句**，禁止 SET NAMES
- 加密: AES-256-GCM，`SREAGENT_SECRET_KEY`（64位hex），格式 `enc:<base64(nonce+密文)>`
- 日志: `zap`，goroutine 内用 `zap.Error` 不用 `zap.Fatal`

**前端**: Vue 3 + Naive UI + Pinia + `useCrudModal`/`usePaginatedList` composable

## 目录

```
cmd/server/main.go           # 入口 + DI wiring
internal/
  model/ (22) handler/ (30) service/ (29) repository/ (21)
  engine/ (6)                # 告警引擎：evaluator + rule_eval + suppression + heartbeat + escalation
  middleware/ (3)            # JWT / CORS / Logger
  router/router.go           # 120+ 端点
  pkg/                       # dbmigrate / datasource / lark / redis / errors
web/src/                     # Vue 3 前端
```

## 错误码

`0` 成功 | `10001` 参数错 | `10002` 业务错 | `10200` 权限不足 | `40001` 未授权 | `50001` DB错 | `50003` 外部API错

## 环境变量

`SREAGENT_DATABASE_PASSWORD` / `SREAGENT_REDIS_PASSWORD` / `SREAGENT_JWT_SECRET` / `SREAGENT_SECRET_KEY`

## 开发命令

`make run` | `make dev` | `make test` | `make lint` | `make web-dev` | `make docker-up`

## 数据模型

```
DataSource ─1:N─ AlertRule ─1:N─ AlertEvent ─1:N─ AlertTimeline
Team ─1:N─ TeamMember ─N:1─ User
EscalationPolicy ─1:N─ EscalationStep
NotifyRule / MuteRule / InhibitionRule / SubscribeRule ── match labels → NotifyMedia
```

## 测试约定

- 框架：Go 标准 `testing` + `testify`
- 工具：`internal/testutil/`（TestDB, SeedUser, SeedAlertRule, CleanupDB）
- 文件：与被测文件同目录，后缀 `_test.go`
- 命名：`Test_{函数名}_{场景}_{预期结果}`
- 新功能必须有测试，Bug 修复必须有回归测试
- 覆盖率目标：service 层 > 60%，handler 层 > 40%

## 自动路由规则（收到需求后自动执行，不需要用户指定）

### 第 1 步：定位模块
根据用户描述的关键词，在 MODULES.md 中找到对应模块。

**关键词 → 模块映射**（命中任一关键词即定位）：
- 告警、规则、事件、firing、resolve、分组、触发、恢复、严重等级、severity、threshold、条件、评估、去重、指纹、心跳、heartbeat、状态机、group_wait、group_interval → 告警引擎 + 告警规则 + 告警事件
- 通知、飞书、邮件、webhook、lark、短信、渠道、分发、模板、订阅、推送、card、卡片、消息、dispatch、notify → 通知管道 + 飞书集成 + 告警通道
- 值班、排班、oncall、替班、升级、轮转、rotation、shift、日程、schedule、escalation、值班人 → 值班排班 + 升级策略
- 静默、mute、屏蔽、抑制、inhibition、silence、免打扰 → 静默规则 + 抑制规则
- 数据源、Prometheus、PromQL、VM、VictoriaMetrics、VLogs、Zabbix、采集、查询、metric、指标 → 数据源 + 标签注册表
- 登录、SSO、OIDC、权限、RBAC、JWT、token、认证、授权、密码、session、Keycloak、角色 → 认证 + 用户管理 + 团队
- 仪表盘、统计、MTTA、MTTR、趋势、top、概览、分析图、dashboard → 仪表盘
- AI、LLM、大模型、分析、根因、SOP、智能、assistant → AI 助手
- 审计、操作日志、audit、变更记录、操作记录 → 审计日志
- 设置、配置、加密、AES、系统、SMTP、全局配置、健康检查 → 系统设置
- 标签、label、tag、自动补全、注册表 → 标签注册表
- 分组、biz-group、业务、树形、scope、作用域 → 业务分组
- 用户、成员、团队、team、member、头像、个人信息、虚拟用户 → 用户管理 + 团队
- 上传、头像、个人设置、偏好 → 用户管理

### 第 2 步：读取上下文（自动，不需要用户指定）
1. 读 MODULES.md 中该模块条目（状态、依赖、文件列表）
2. 读 `docs/{module}.md`（如果存在）
3. 读该模块的代码文件（从 MODULES.md 文件列表获取路径）
4. 读相关测试文件（`_test.go`）

### 第 3 步：检查依赖
从 MODULES.md 依赖图确认修改是否影响其他模块。有影响则先告知用户影响范围。

### 第 4 步：检查测试
- 有测试 → 读取现有测试了解覆盖范围
- 无测试 → 告知用户，建议实现后补测试

### 第 5 步：给方案
基于以上信息给出实现方案（修改哪些文件、新增哪些代码）。用户确认后再动手。

## 变更追踪规则（每次修改后自动执行，不需要用户提醒）

**所有变更必做**：在 CHANGELOG.md 顶部追加记录

### 新增模块
1. MODULES.md 添加完整条目（功能、文件、API、依赖、状态、测试覆盖）
2. 创建 docs/{module}.md
3. 自动路由规则关键词映射添加对应关键词
4. MODULES.md 依赖关系图添加该模块

### 删除/合并模块
1. MODULES.md 删除/合并对应条目、依赖图、测试覆盖表
2. 删除/合并 docs/{module}.md
3. 自动路由规则关键词映射删除/迁移对应关键词

### API 变更
1. 新增端点 → 更新 docs/api.md
2. 删除端点 → 从 docs/api.md 移除
3. 修改端点（参数/响应）→ 更新 docs/api.md 对应段落

### 数据模型变更
1. 新增/修改字段 → 更新 MODULES.md 数据模型关系
2. 新增迁移文件 → 遵循 `migrations/{序号}_{描述}.{up|down}.sql` 规范
3. CHANGELOG 该版本条目中注明迁移文件编号（如 `迁移: 000015_xxx`）
4. 必须同时提供 up + down 两个文件，down 必须能干净回退

### 测试变更
1. 新增测试 → 更新 MODULES.md 测试覆盖表（状态 + 覆盖率）
2. Bug 修复带回归测试 → 同步更新覆盖表

### 配置变更
1. 新增/修改环境变量 → 更新 CLAUDE.md 环境变量段
2. 新增配置项 → 更新 config.example.yaml

### 版本发布（发版 checklist）
1. 更新 CLAUDE.md 头部版本号
2. 更新 MODULES.md 头部版本号
3. 同步 `web/package.json` 的 version 字段（侧边栏版本号从此读取）
4. 确认所有新增迁移文件有对应的 down.sql
5. CHANGELOG 该版本条目列出所有迁移文件编号
6. `go build` + `go test` 通过
7. `cd web && npx vite build` 通过
8. 标注该版本依赖的最低迁移版本（方便回滚判断）

## 对话规范（自动生效）
1. 完成后 `go build` 通过 + 自动执行变更追踪规则
2. 超过 20 轮对话考虑开新会话
