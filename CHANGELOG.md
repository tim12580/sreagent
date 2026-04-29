# 变更日志 (CHANGELOG)

> 基于 git tag 和 commit 记录整理。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)

---

## [v1.16.0] - 2026-04-29

### Added
- 日志探索页面（Log Explorer）：LogsQL 查询、日志表格展示、时间范围选择
- VictoriaLogs 日志查询端点：`POST /api/v1/datasources/:id/log-query`
- 侧栏菜单新增「日志探索」入口（数据源子菜单下）
- 中英文 i18n 支持

### Fixed
- 修复数据查询页白屏：`crypto.randomUUID` 在 HTTP 非安全上下文下不可用，改用 fallback UUID 生成
- 修复登录页 401 错误显示英文：拦截器现在优先使用后端返回的业务错误码进行本地化（如 10102 → "用户名或密码错误"）

## [v1.15.0] - 2026-04-29

### Added
- 可编程告警处理链（Event Pipeline）：DAG 可视化编辑器 + 5 种处理器
- 处理器：If（条件分支）、Relabel（标签操作）、EventDrop（告警丢弃）、Callback（Webhook 回调）、AISummary（AI 摘要）
- Pipeline CRUD 端点：`/api/v1/event-pipelines`（7 个端点）
- Pipeline 试运行：`POST /api/v1/event-pipelines/tryrun`
- Pipeline 执行记录：`GET /api/v1/event-pipelines/:id/executions`
- 前端 Pipeline 列表页 + DAG 编辑器（原生 SVG + 拖拽连线）
- 前端节点配置面板（右侧抽屉，支持各处理器类型专属配置）
- Pipeline 引擎集成到 onAlertFn（inhibition → mute → bizgroup → **pipeline** → notify）
- 迁移: 000017_event_pipelines

## [v1.14.0] - 2026-04-29

### Added
- 数据源探索页面（Explore）：PromQL 编辑器（CodeMirror 6 + 语法高亮 + 自动补全）
- Range Query 支持：POST /api/v1/datasources/:id/query-range
- 数据源标签代理端点：GET /api/v1/datasources/:id/labels/keys、labels/values、metrics
- ECharts 时间序列图表（dataZoom、tooltip cross 指针、Legend 统计表格）
- 时间范围选择器（相对/绝对时间）+ 自动刷新
- 多查询支持、Legend 格式化、Chart/Table 视图切换
- 仪表盘 V2 系统：Dashboard CRUD 端点（/api/v1/dashboards）
- 变量模板系统：query/custom/textbox/constant 类型，$var 替换
- 仪表盘列表页和查看页（全局时间范围、变量选择器）
- 值格式化工具（bytes/seconds/percent/short/scientific）
- 迁移: 000016_dashboards

### Changed
- /datasources/query 路由指向新的 Explore 页面（替代原生 HTML 查询页）

## [v1.11.0] - 2026-04-27

### Added
- 登录页密码/用户名错误 inline 提示（表单内 alert 替代右上角 message）
- 数据源卡片显示版本号（健康检查成功后持久化 version 字段）
- 数据源状态标签国际化（healthy/unhealthy/unknown 随语言切换）
- 密码复杂度校验（最少 8 位，含大小写字母和数字）
- JWT 超时可配置（系统设置 > 安全配置，预设 1h/4h/8h/24h/7d）
- 数据源查询页面（选择数据源 + 输入 PromQL/LogQL 执行查询）
- 迁移: 000015_datasource_version

### Removed
- 登录页默认账号 admin/admin123 提示

### Changed
- AuthService.Login / RefreshToken 读取 system_settings 中的 jwt_expire_seconds
- handler/auth.go, handler/user.go 密码最小长度约束从 6 提升至 8

## [v1.10.0] - 2026-04-26

### Added
- 测试框架：internal/testutil/ (TestDB, SeedUser, SeedAlertRule, CleanupDB)
- 测试骨架：service/alert_channel_test.go, handler/alert_channel_test.go
- docs/testing.md 测试策略和覆盖目标
- docs/prompts.md AI 提示词模板（新功能/Bug/审查/测试等）
- CLAUDE.md 对话规范（token 节省规则）
- config.example.yaml OIDC 配置段
- GET /schedules/:id/participants 后端 handler + 路由
- GET /schedules/:id/overrides 后端 handler + 路由
- POST /alert-channels/:id/test 后端 handler + 路由

### Fixed
- 修复 3 个前端 API 调用无后端路由的问题（schedule participants/overrides, alert-channel test）

### Removed
- 4 个孤立 Vue 组件（SpotlightCursor, SeverityTag, StatusTag, SkeletonCard）
- 废弃 TS 类型（NotifyChannel, NotifyPolicy v1）
- 无关文档（3th_monitor_readme.md）
- scripts/test-api.sh 中的 v1 通知端点

## [v1.9.10] - 2026-04-26

### Fixed
- label_registry.label_value 从 VARCHAR(512) 扩展到 VARCHAR(2048)，修复 Prometheus 长标签值导致 MySQL Error 1406
- SyncDatasource / RecordFromLabels 添加 >2048 截断安全网
- **迁移**: 000014_label_value_extend

## [v1.9.9] - 2026-04-26

### Added
- Alertmanager 风格 group_wait / group_interval 通知分组
- AlertGroupManager 在引擎回调和 RouteAlert 之间缓冲 firing 事件
- AlertRule 新增 group_wait_seconds / group_interval_seconds 字段
- 前端告警规则表单新增分组等待/间隔配置
- **迁移**: 000013_alert_rule_group_timing

## [v1.9.8] - 2026-04-25

### Added
- CLAUDE.md 与 .opencode/context.md 合并为单一 AI 导航文件
- .gitignore 添加 .claude/ 和 .opencode/ 排除
- Claude Code 全局 settings.json 权限配置

## [v1.6.0] - 2026-04-20

### Added
- 系统级 SMTP 配置（system_settings group=smtp）
- 升级策略 email 分支接入系统 SMTP 真实发送
- JWT 7天宽限续签（POST /auth/refresh）
- 前端 Axios 401 自动刷新 token
- 头像 Go 层大小校验（≤272KB data URL）
- GET /alert-events/export CSV 流式导出
- GET /mute-rules/preview 命中预览
- Lark OpenID → DB User 映射（user.lark_user_id）
- 个人设置新增「飞书账号绑定」tab
- 数据源健康检查返回 latency/version 富结果
- **迁移**: 000008_create_inhibition_rules, 000009_create_label_registry, 000010_alert_rule_datasource_optional, 000011_alert_rule_datasource_type

## [v1.5.0] - 2026-04-15

### Added
- 升级策略 lark_personal 分支接入 Lark Bot API（DM）
- 告警 AutoResolve 时同步 PATCH Lark 卡片
- LarkBotService.SendMessage 优先用 Bot API 回复 chatID
- NotifyChannel Bot API 类型在 TestChannel 支持真发送
- **迁移**: 000006_heartbeat_sla_alert_rules, 000007_sla_escalated_at_alert_events

## [v1.3.1] - 2026-04-10

### Added
- MTTA/MTTR P50/P95 百分位、按严重程度细分
- MTTA/MTTR 每日趋势折线图
- 品牌 logo.svg（sider/login/favicon 统一）
- 个人信息头像扩展为 32 个预设 emoji + 自定义上传

### Fixed
- 顶部栏保存头像后仍显示用户名首字母

## [v1.3.0] - 2026-04-08

### Changed
- 设计系统级视觉翻新：CSS token + Naive UI GlobalThemeOverrides
- 侧栏/顶栏/登录页玻璃态皮肤（dark + light）

## [v1.2.0] - 2026-04-05

### Added
- 告警规则分类 tab
- 仪表盘分析图表（趋势 + Top 规则）
- 操作审计日志
- 表达式实时测试
- **迁移**: 000004_audit_logs, 000005_add_rule_category

## [v1.1.x] - 2026-04-01

### Added
- 告警详情页改版（严重等级横幅 + 生命周期时间线）
- 通知模块合并为单页 Tabs
- **迁移**: 000003_alert_event_lark_message_id

## [v1.0.x] - 2026-03-25

### Added
- OIDC 配置 UI（存 DB）
- K8s 清单
- 多数据源集成
- RBAC 三级权限
- **迁移**: 000001_initial_schema, 000002_system_settings

> Phase 追踪和 QA 修复汇总已移至 [docs/phases.md](docs/phases.md)
