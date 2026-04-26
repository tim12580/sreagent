# 变更日志 (CHANGELOG)

> 基于 git tag 和 commit 记录整理。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)

---

## [v1.9.10] - 2026-04-26

### Fixed
- label_registry.label_value 从 VARCHAR(512) 扩展到 VARCHAR(2048)，修复 Prometheus 长标签值导致 MySQL Error 1406
- SyncDatasource / RecordFromLabels 添加 >2048 截断安全网

## [v1.9.9] - 2026-04-26

### Added
- Alertmanager 风格 group_wait / group_interval 通知分组
- AlertGroupManager 在引擎回调和 RouteAlert 之间缓冲 firing 事件
- AlertRule 新增 group_wait_seconds / group_interval_seconds 字段
- 前端告警规则表单新增分组等待/间隔配置

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

## [v1.5.0] - 2026-04-15

### Added
- 升级策略 lark_personal 分支接入 Lark Bot API（DM）
- 告警 AutoResolve 时同步 PATCH Lark 卡片
- LarkBotService.SendMessage 优先用 Bot API 回复 chatID
- NotifyChannel Bot API 类型在 TestChannel 支持真发送

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

## [v1.1.x] - 2026-04-01

### Added
- 告警详情页改版（严重等级横幅 + 生命周期时间线）
- 通知模块合并为单页 Tabs

## [v1.0.x] - 2026-03-25

### Added
- OIDC 配置 UI（存 DB）
- K8s 清单
- 多数据源集成
- RBAC 三级权限

---

## Phase 记录

| 阶段 | 内容 | 状态 |
|------|------|:----:|
| Phase 0 | Cleanup（删除遗留文件、修复 Dockerfile/K8s 配置） | ✅ |
| Phase 1 | CI/CD 完整文档 | ✅ |
| Phase 2 | Redis 引擎状态持久化 | ✅ |
| Phase 3 | Keycloak OIDC + RBAC 权限 | ✅ |
| Phase 4 | 核心模块补完（Subscribe/Notify 管道） | ✅ |
| Phase 5 | 前端 UI 全面改版（7 个子阶段） | ✅ |
| Phase 6 | API 文档（120+ 端点） | ✅ |
| Phase 7 | QA 多角色验证（14 后端 + 16 前端修复） | ✅ |
| Phase 8 | 上下文压缩 + 文档更新 | ✅ |

## QA 修复汇总 (Phase 7)

### 后端（14 项）
- RequireRole / GetCurrentUserID 不安全类型断言 → comma-ok
- ChangePassword 修改了管理员自己的密码 → 改用 URL :id 参数
- OIDC callback 无 CSRF state 验证 → 添加 state cookie 验证
- OIDC Secure cookie flag 硬编码 → 从 TLS/X-Forwarded-Proto 推导
- OIDC JWT 通过 query param 传递 → 改为 URL fragment
- Redis 在 HTTP Server 之前关闭 → 调换顺序
- zap.Fatal 在 goroutine 中阻止优雅关闭 → zap.Error + os.Exit
- StateEntry 缺少 Annotations → 添加字段

### 前端（16 项）
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
