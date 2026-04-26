# SREAgent REST API 参考手册

> 基于源码自动生成。最后更新：2026-04-04。

## 目录

- [约定](#约定)
- [认证](#1-认证)
- [OIDC 单点登录](#2-oidc-单点登录)
- [数据源](#3-数据源)
- [告警规则](#4-告警规则)
- [告警事件](#5-告警事件)
- [静默规则](#6-静默规则)
- [通知规则（v2）](#7-通知规则v2)
- [通知媒介](#8-通知媒介)
- [消息模板](#9-消息模板)
- [订阅规则](#10-订阅规则)
- [业务分组](#11-业务分组)
- [告警通道](#12-告警通道)
- [用户通知配置](#13-用户通知配置)
- [通知渠道（v1）](#14-通知渠道v1)
- [通知策略（v1）](#15-通知策略v1)
- [用户](#16-用户)
- [团队](#17-团队)
- [值班管理](#18-值班管理)
- [升级策略](#19-升级策略)
- [AI](#20-ai)
- [飞书机器人](#21-飞书机器人)
- [引擎](#22-引擎)
- [仪表盘](#23-仪表盘)
- [Webhook](#24-webhook)
- [告警操作页面](#25-告警操作页面)

---

## 约定

### 基础 URL

所有 API 路由均以 `/api/v1` 为前缀，另有说明除外。

### 统一响应格式

所有 JSON 端点返回统一的响应信封：

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... }
}
```

- `code = 0` — 成功
- `code != 0` — 错误（message 字段包含可读的错误描述）

### 分页

分页列表端点接受以下参数：

| 参数 | 类型 | 默认值 | 范围 | 说明 |
|------|------|--------|------|------|
| `page` | int | 1 | >= 1 | 页码 |
| `page_size` | int | 20 | 1–100 | 每页条数 |

分页响应的 `data` 结构如下：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [ ... ],
    "total": 128,
    "page": 1,
    "page_size": 20
  }
}
```

### 认证

受保护的路由需要在 `Authorization` 请求头中携带 JWT 令牌：

```
Authorization: Bearer <token>
```

令牌通过 `POST /api/v1/auth/login` 或 OIDC 回调流程获取。

### RBAC 角色

五种角色，按权限从高到低排列：

| 角色 | 说明 |
|------|------|
| `admin` | 拥有所有资源的完全访问权限 |
| `team_lead` | 管理配置对象（规则、渠道、排班、团队） |
| `member` | 执行操作（确认、解决、订阅） |
| `viewer` | 对分配的资源拥有只读权限 |
| `global_viewer` | 对所有资源拥有只读权限 |

下文引用的路由访问级别说明：
- **公开** — 无需认证
- **已认证** — 任何已认证用户
- **操作权限** — `admin`、`team_lead` 或 `member`
- **管理权限** — `admin` 或 `team_lead`
- **仅管理员** — 仅 `admin`

### 通用模型字段

所有实体均包含以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | 自增主键 |
| `created_at` | datetime | ISO 8601 格式 |
| `updated_at` | datetime | ISO 8601 格式 |

---

## 1. 认证

### POST `/api/v1/auth/login` — 登录

**访问级别：** 公开

**请求体：**

```json
{
  "username": "admin",
  "password": "secret123"
}
```

**响应：**

```json
{
  "code": 0,
  "data": {
    "token": "eyJhbGciOi...",
    "expires_in": 86400
  }
}
```

### GET `/api/v1/auth/profile` — 获取当前用户信息

**访问级别：** 已认证

**响应：** 用户对象（详见 [用户](#16-用户) 模型）。响应中不包含密码字段。

### PUT `/api/v1/me/profile` — 更新个人资料

**访问级别：** 已认证

| 字段 | 类型 | 说明 |
|------|------|------|
| `display_name` | string | 显示名称 |
| `email` | string | 邮箱 |
| `phone` | string | 手机号 |
| `avatar` | string | Base64 data URL 或预设头像标识 |

### POST `/api/v1/me/password` — 修改个人密码

**访问级别：** 已认证

| 字段 | 类型 | 必填 | 校验规则 |
|------|------|------|----------|
| `old_password` | string | 是 | |
| `new_password` | string | 是 | 至少 6 个字符 |

---

## 2. OIDC 单点登录

### GET `/api/v1/auth/oidc/config` — OIDC 状态

**访问级别：** 公开

**响应：**

```json
{
  "code": 0,
  "data": {
    "enabled": true,
    "login_url": "/api/v1/auth/oidc/login"
  }
}
```

未配置 OIDC 时返回 `{"enabled": false}`。

### GET `/api/v1/auth/oidc/login` — 发起 OIDC 登录

**访问级别：** 公开

重定向（302）到已配置的身份提供商授权端点。设置 `oidc_state` Cookie 用于 CSRF 防护。

### GET `/api/v1/auth/oidc/callback` — OIDC 回调

**访问级别：** 公开

由身份提供商在认证完成后调用。成功后将浏览器重定向到：

```
/?oidc_token=<jwt>&expires_in=<seconds>
```

前端路由守卫拦截 `oidc_token` 查询参数并存储令牌。

**查询参数：**

| 参数 | 说明 |
|------|------|
| `code` | 身份提供商返回的授权码 |
| `state` | CSRF 状态值（与 Cookie 中的值进行校验） |
| `error` | 错误码（可选） |
| `error_description` | 错误详情（可选） |

### POST `/api/v1/auth/oidc/token` — 用授权码换取令牌（JSON）

**访问级别：** 公开

适用于偏好 JSON 流程而非重定向的 SPA 客户端。

**请求体：**

```json
{ "code": "abc123" }
```

**响应：** 与登录响应相同（`token`、`expires_in`）。

---

## 3. 数据源

管理 Prometheus、VictoriaMetrics、VictoriaLogs 和 Zabbix 数据源。

**模型字段：** `name`、`type`（prometheus | victoriametrics | zabbix | victorialogs）、`endpoint`、`description`、`labels`（map）、`status`（healthy | unhealthy | unknown）、`auth_type`（none | basic | bearer | api_key）、`auth_config`（JSON）、`health_check_interval`、`is_enabled`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/datasources` | 已认证 | 列表（分页）。筛选：`?type=prometheus` |
| GET | `/datasources/:id` | 已认证 | 按 ID 获取 |
| POST | `/datasources` | 仅管理员 | 创建 |
| PUT | `/datasources/:id` | 仅管理员 | 更新 |
| DELETE | `/datasources/:id` | 仅管理员 | 删除 |
| POST | `/datasources/:id/health-check` | 管理权限 | 触发健康检查 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 唯一名称 |
| `type` | string | 是 | 支持的类型之一 |
| `endpoint` | string | 是 | URL 地址 |
| `description` | string | 否 | 描述 |
| `labels` | map[string]string | 否 | 键值对元数据 |
| `auth_type` | string | 否 | 默认：`none` |
| `auth_config` | string (JSON) | 否 | 认证相关配置 |
| `health_check_interval` | int | 否 | 健康检查间隔（秒） |

**健康检查响应：**

```json
{ "code": 0, "data": { "status": "healthy" } }
```

---

## 4. 告警规则

使用 PromQL、LogsQL 或其他查询表达式定义评估规则。

**模型字段：** `name`、`display_name`、`description`、`datasource_id`、`expression`、`for_duration`、`severity`（critical | warning | info）、`labels`（map）、`annotations`（map）、`status`（enabled | disabled | muted）、`group_name`、`version`、`eval_interval`、`recovery_hold`、`nodata_enabled`、`nodata_duration`、`suppress_enabled`、`biz_group_id`、`created_by`、`updated_by`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/alert-rules` | 已认证 | 列表（分页）。筛选：`?severity=critical&status=enabled&group_name=infra` |
| GET | `/alert-rules/:id` | 已认证 | 按 ID 获取 |
| GET | `/alert-rules/export` | 已认证 | 导出为 YAML。筛选：`?group_name=infra` |
| POST | `/alert-rules` | 管理权限 | 创建 |
| PUT | `/alert-rules/:id` | 管理权限 | 更新 |
| DELETE | `/alert-rules/:id` | 管理权限 | 删除 |
| PATCH | `/alert-rules/:id/status` | 管理权限 | 切换状态 |
| POST | `/alert-rules/import` | 管理权限 | 从 YAML/JSON 文件导入 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 规则标识符 |
| `display_name` | string | 否 | 人类可读的名称 |
| `description` | string | 否 | 描述 |
| `datasource_id` | uint | 是 | 关联数据源 ID |
| `expression` | string | 是 | PromQL / LogsQL 表达式 |
| `for_duration` | string | 否 | 例如 `"5m"` |
| `severity` | string | 是 | critical、warning、info |
| `labels` | map[string]string | 否 | 附加标签 |
| `annotations` | map[string]string | 否 | 注解（摘要、描述） |
| `group_name` | string | 否 | 规则分组 |
| `eval_interval` | int | 否 | 评估间隔（秒） |
| `recovery_hold` | int | 否 | 自动恢复前的等待时间（秒） |
| `nodata_enabled` | bool | 否 | 数据缺失时是否触发告警 |
| `nodata_duration` | int | 否 | 数据缺失阈值（秒） |
| `suppress_enabled` | bool | 否 | 启用基于级别的抑制 |
| `biz_group_id` | *uint | 否 | 所属业务分组 |

**切换状态请求体：**

```json
{ "status": "enabled" }
```

**导入** — `multipart/form-data`：

| 字段 | 类型 | 说明 |
|------|------|------|
| `file` | file | `.yaml` / `.yml` / `.json`（Prometheus 规则文件格式） |
| `datasource_id` | string | 导入规则的默认数据源 |

**导入响应：**

```json
{ "code": 0, "data": { "total": 10, "success": 9, "failed": 1, "errors": ["..."] } }
```

**导出** — 返回 `application/x-yaml` Content-Type，带有 `Content-Disposition: attachment` 头。

---

## 5. 告警事件

由评估引擎生成或通过 Webhook 接收的实时和历史告警实例。

**模型字段：** `fingerprint`、`rule_id`、`alert_name`、`severity`、`status`（firing | acknowledged | assigned | silenced | resolved | closed）、`labels`（map）、`annotations`（map）、`source`、`generator_url`、`fired_at`、`acked_at`、`resolved_at`、`closed_at`、`acked_by`、`assigned_to`、`silenced_until`、`silence_reason`、`resolution`、`fire_count`、`oncall_user_id`、`is_dispatched`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/alert-events` | 已认证 | 列表（分页）。筛选：`?status=firing&severity=critical&view_mode=mine` |
| GET | `/alert-events/:id` | 已认证 | 按 ID 获取 |
| GET | `/alert-events/:id/timeline` | 已认证 | 获取事件时间线（状态变更、评论） |
| POST | `/alert-events/:id/acknowledge` | 操作权限 | 确认告警 |
| POST | `/alert-events/:id/assign` | 操作权限 | 指派给用户 |
| POST | `/alert-events/:id/resolve` | 操作权限 | 解决告警 |
| POST | `/alert-events/:id/close` | 操作权限 | 关闭告警 |
| POST | `/alert-events/:id/comment` | 操作权限 | 添加评论 |
| POST | `/alert-events/:id/silence` | 操作权限 | 静默告警 |
| POST | `/alert-events/batch/acknowledge` | 操作权限 | 批量确认 |
| POST | `/alert-events/batch/close` | 操作权限 | 批量关闭 |

**列表筛选参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `status` | string | firing、acknowledged、assigned、silenced、resolved、closed |
| `severity` | string | critical、warning、info |
| `view_mode` | string | `mine`（指派给我）、`unassigned`（未指派）、`all`（默认） |
| `user_id` | uint | 管理员可用此参数覆盖 view_mode=mine |

**指派请求体：**

```json
{ "assign_to": 5, "note": "Please investigate" }
```

**解决请求体：**

```json
{ "resolution": "Fixed the root cause by scaling the service" }
```

**关闭请求体：**

```json
{ "note": "False positive" }
```

**评论请求体：**

```json
{ "note": "Investigating the issue now" }
```

**静默请求体：**

```json
{ "duration_minutes": 60, "reason": "Maintenance window" }
```

**批量确认 / 关闭请求体：**

```json
{ "ids": [1, 2, 3] }
```

**批量操作响应：**

```json
{ "code": 0, "data": { "success": 3, "failed": 0 } }
```

---

## 6. 静默规则

在指定时间窗口内，对匹配特定条件的告警抑制通知。

**模型字段：** `name`、`description`、`match_labels`（map）、`severities`（逗号分隔）、`start_time`、`end_time`、`periodic_start`、`periodic_end`、`days_of_week`、`timezone`、`is_enabled`、`rule_ids`（逗号分隔）、`created_by`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/mute-rules` | 已认证 | 列表（分页） |
| GET | `/mute-rules/:id` | 已认证 | 按 ID 获取 |
| POST | `/mute-rules` | 管理权限 | 创建 |
| PUT | `/mute-rules/:id` | 管理权限 | 更新 |
| DELETE | `/mute-rules/:id` | 管理权限 | 删除 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `match_labels` | map[string]string | 否 | 标签匹配器 |
| `severities` | string | 否 | 例如 `"critical,warning"` |
| `start_time` | datetime | 否 | 一次性窗口开始时间（ISO 8601） |
| `end_time` | datetime | 否 | 一次性窗口结束时间 |
| `periodic_start` | string | 否 | 每日开始时间，例如 `"02:00"` |
| `periodic_end` | string | 否 | 每日结束时间，例如 `"06:00"` |
| `days_of_week` | string | 否 | 例如 `"1,2,3,4,5"`（周一=1） |
| `timezone` | string | 否 | 默认：`"Asia/Shanghai"` |
| `is_enabled` | bool | 否 | 是否启用 |
| `rule_ids` | string | 否 | 逗号分隔的告警规则 ID |

---

## 7. 通知规则（v2）

支持管道处理和按规则配置通知目标的高级通知规则。

**模型字段：** `name`、`description`、`is_enabled`、`severities`、`match_labels`（map）、`pipeline`（JSON）、`notify_configs`（JSON）、`repeat_interval`、`callback_url`、`created_by`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/notify-rules` | 已认证 | 列表（分页） |
| GET | `/notify-rules/:id` | 已认证 | 按 ID 获取 |
| POST | `/notify-rules` | 管理权限 | 创建 |
| PUT | `/notify-rules/:id` | 管理权限 | 更新 |
| DELETE | `/notify-rules/:id` | 管理权限 | 删除 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `is_enabled` | bool | 否 | 是否启用 |
| `severities` | string | 否 | 逗号分隔 |
| `match_labels` | map[string]string | 否 | 标签匹配器 |
| `pipeline` | string (JSON) | 否 | 处理管道步骤 |
| `notify_configs` | string (JSON) | 否 | 通知配置数组 |
| `repeat_interval` | int | 否 | 重复通知间隔（秒） |
| `callback_url` | string | 否 | Webhook 回调 URL |

---

## 8. 通知媒介

通知媒介（投递渠道）：飞书 Webhook、邮件、HTTP Webhook、脚本。

**模型字段：** `name`、`type`（lark_webhook | email | http | script）、`description`、`is_enabled`、`config`（JSON）、`variables`（JSON）、`is_builtin`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/notify-media` | 已认证 | 列表（分页） |
| GET | `/notify-media/:id` | 已认证 | 按 ID 获取 |
| POST | `/notify-media` | 管理权限 | 创建 |
| PUT | `/notify-media/:id` | 管理权限 | 更新 |
| DELETE | `/notify-media/:id` | 管理权限 | 删除 |
| POST | `/notify-media/:id/test` | 管理权限 | 发送测试通知 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `type` | string | 是 | 类型：lark_webhook、email、http、script |
| `description` | string | 否 | 描述 |
| `is_enabled` | bool | 否 | 是否启用 |
| `config` | string (JSON) | 是 | 类型专属配置 |
| `variables` | string (JSON) | 否 | 模板变量 |

**测试响应：**

```json
{ "code": 0, "data": { "message": "test notification sent" } }
```

---

## 9. 消息模板

基于 Go `text/template` 的通知消息模板。

**模型字段：** `name`、`description`、`content`（Go 模板字符串）、`type`（text | html | markdown | lark_card）、`is_builtin`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/message-templates` | 已认证 | 列表（分页） |
| GET | `/message-templates/:id` | 已认证 | 按 ID 获取 |
| POST | `/message-templates` | 管理权限 | 创建 |
| PUT | `/message-templates/:id` | 管理权限 | 更新 |
| DELETE | `/message-templates/:id` | 管理权限 | 删除 |
| POST | `/message-templates/preview` | 已认证 | 预览模板渲染结果 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `content` | string | 是 | Go 模板字符串 |
| `type` | string | 否 | 默认：`"text"` |

**预览请求体：**

```json
{ "content": "Alert {{ .AlertName }} is {{ .Status }}" }
```

**预览响应：**

```json
{ "code": 0, "data": { "rendered": "Alert CPUHigh is firing" } }
```

---

## 10. 订阅规则

允许用户/团队订阅匹配特定条件的告警，并将其路由到指定的通知规则。

**模型字段：** `name`、`description`、`is_enabled`、`match_labels`（map）、`severities`、`notify_rule_id`、`user_id`、`team_id`、`created_by`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/subscribe-rules` | 已认证 | 列表（分页） |
| GET | `/subscribe-rules/:id` | 已认证 | 按 ID 获取 |
| POST | `/subscribe-rules` | 操作权限 | 创建 |
| PUT | `/subscribe-rules/:id` | 操作权限 | 更新 |
| DELETE | `/subscribe-rules/:id` | 操作权限 | 删除 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `is_enabled` | bool | 否 | 是否启用 |
| `match_labels` | map[string]string | 否 | 标签匹配器 |
| `severities` | string | 否 | 逗号分隔 |
| `notify_rule_id` | uint | 是 | 目标通知规则 |
| `user_id` | uint | 否 | 为指定用户订阅 |
| `team_id` | uint | 否 | 为团队订阅 |

---

## 11. 业务分组

用于组织告警规则和访问控制的层级业务分组树。

**模型字段：** `name`（支持 `/` 表示层级）、`description`、`parent_id`、`labels`（map）、`members`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/biz-groups` | 已认证 | 列表（分页） |
| GET | `/biz-groups/tree` | 已认证 | 获取树形结构 |
| GET | `/biz-groups/:id` | 已认证 | 按 ID 获取 |
| GET | `/biz-groups/:id/members` | 已认证 | 列出分组成员 |
| POST | `/biz-groups` | 管理权限 | 创建 |
| PUT | `/biz-groups/:id` | 管理权限 | 更新 |
| DELETE | `/biz-groups/:id` | 管理权限 | 删除 |
| POST | `/biz-groups/:id/members` | 管理权限 | 添加成员 |
| DELETE | `/biz-groups/:id/members/:uid` | 管理权限 | 移除成员 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `parent_id` | uint | 否 | 父分组 ID |
| `labels` | map[string]string | 否 | 标签 |

**添加成员请求体：**

```json
{ "user_id": 5, "role": "admin" }
```

角色可选 `"admin"` 或 `"member"`。

---

## 12. 告警通道

虚拟告警路由通道，将通知媒介与可选的模板和标签匹配器绑定。

**模型字段：** `name`、`description`、`match_labels`（map）、`severities`、`media_id`、`template_id`、`throttle_min`、`is_enabled`、`created_by`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/alert-channels` | 已认证 | 列表（分页） |
| GET | `/alert-channels/:id` | 已认证 | 按 ID 获取 |
| POST | `/alert-channels` | 管理权限 | 创建 |
| PUT | `/alert-channels/:id` | 管理权限 | 更新 |
| DELETE | `/alert-channels/:id` | 管理权限 | 删除 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `match_labels` | map[string]string | 否 | 标签匹配器 |
| `severities` | string | 否 | 逗号分隔 |
| `media_id` | uint | 是 | 关联通知媒介 ID |
| `template_id` | uint | 否 | 关联消息模板 ID |
| `throttle_min` | int | 否 | 通知最小间隔（分钟） |
| `is_enabled` | bool | 否 | 是否启用 |

---

## 13. 用户通知配置

当前用户的个人通知偏好设置（多媒介）。

**模型字段：** `user_id`、`media_type`（lark_personal | email | webhook）、`config`（JSON）、`is_enabled`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/me/notify-configs` | 已认证 | 列出当前用户的配置 |
| PUT | `/me/notify-configs` | 已认证 | 创建或更新（按 media_type upsert） |
| DELETE | `/me/notify-configs/:mediaType` | 已认证 | 按媒介类型删除 |

**Upsert 请求体：**

```json
{
  "media_type": "email",
  "config": "{\"address\": \"user@example.com\"}",
  "is_enabled": true
}
```

**删除路径参数：** `:mediaType` — 例如 `email`、`lark_personal`、`webhook`。

---

## 14. 通知渠道（v1）

旧版通知渠道。类型：`lark_webhook`、`lark_bot`、`email`、`sms`、`custom_webhook`。

**模型字段：** `name`、`type`、`description`、`labels`（map）、`config`（JSON，GET 时隐藏）、`is_enabled`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/notify-channels` | 已认证 | 列表（分页） |
| GET | `/notify-channels/:id` | 已认证 | 按 ID 获取 |
| POST | `/notify-channels` | 管理权限 | 创建 |
| PUT | `/notify-channels/:id` | 管理权限 | 更新 |
| DELETE | `/notify-channels/:id` | 管理权限 | 删除 |
| POST | `/notify-channels/:id/test` | 管理权限 | 发送测试通知 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `type` | string | 是 | 渠道类型 |
| `description` | string | 否 | 描述 |
| `labels` | map[string]string | 否 | 标签 |
| `config` | string (JSON) | 是 | 类型专属配置 |
| `is_enabled` | bool | 否 | 是否启用 |

---

## 15. 通知策略（v1）

旧版通知策略，根据标签匹配将告警路由到通知渠道。

**模型字段：** `name`、`description`、`match_labels`（map）、`severities`、`channel_id`、`throttle_minutes`、`template_name`、`is_enabled`、`priority`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/notify-policies` | 已认证 | 列表（分页） |
| GET | `/notify-policies/:id` | 已认证 | 按 ID 获取 |
| POST | `/notify-policies` | 管理权限 | 创建 |
| PUT | `/notify-policies/:id` | 管理权限 | 更新 |
| DELETE | `/notify-policies/:id` | 管理权限 | 删除 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `match_labels` | map[string]string | 是 | 标签匹配器 |
| `severities` | string | 否 | 逗号分隔 |
| `channel_id` | uint | 是 | 关联通知渠道 ID |
| `throttle_minutes` | int | 否 | 节流窗口（分钟） |
| `template_name` | string | 否 | 默认：`"default"` |
| `is_enabled` | bool | 否 | 是否启用 |
| `priority` | int | 否 | 值越小优先级越高 |

---

## 16. 用户

用户管理。支持普通用户、机器人用户和渠道（虚拟）用户。

**模型字段：** `username`、`display_name`、`email`、`phone`、`lark_user_id`、`avatar`、`role`（admin | team_lead | member | viewer | global_viewer）、`is_active`、`user_type`（human | bot | channel）、`notify_target`（JSON）、`oidc_subject`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/users` | 已认证 | 列表（分页）。筛选：`?user_type=human` |
| GET | `/users/:id` | 已认证 | 按 ID 获取 |
| POST | `/users` | 仅管理员 | 创建普通用户 |
| POST | `/users/virtual` | 仅管理员 | 创建虚拟用户（机器人/渠道） |
| PUT | `/users/:id` | 仅管理员 | 更新用户 |
| PATCH | `/users/:id/active` | 仅管理员 | 启用 / 禁用用户 |
| PATCH | `/users/:id/password` | 仅管理员 | 管理员重置密码 |
| DELETE | `/users/:id` | 仅管理员 | 删除用户 |

**创建普通用户请求体：**

| 字段 | 类型 | 必填 | 校验规则 |
|------|------|------|----------|
| `username` | string | 是 | 唯一 |
| `password` | string | 是 | 至少 6 个字符 |
| `display_name` | string | 否 | |
| `email` | string | 否 | 邮箱格式 |
| `phone` | string | 否 | |
| `lark_user_id` | string | 否 | |
| `avatar` | string | 否 | |
| `role` | string | 否 | 默认：`"member"` |

**创建虚拟用户请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 是 | 用户名 |
| `display_name` | string | 否 | 显示名称 |
| `user_type` | string | 是 | `"bot"` 或 `"channel"` |
| `notify_target` | string | 否 | JSON 通知目标配置 |
| `description` | string | 否 | 描述 |
| `role` | string | 否 | 角色 |

**更新请求体：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `display_name` | string | 显示名称 |
| `email` | string | 邮箱 |
| `phone` | string | 手机号 |
| `lark_user_id` | string | 飞书用户 ID |
| `avatar` | string | 头像 |
| `role` | string | 角色 |

**切换启用状态请求体：**

```json
{ "is_active": true }
```

**修改密码请求体：**

| 字段 | 类型 | 必填 | 校验规则 |
|------|------|------|----------|
| `old_password` | string | 是 | |
| `new_password` | string | 是 | 至少 6 个字符 |

---

## 17. 团队

团队管理，支持成员角色设置。

**模型字段：** `name`、`description`、`labels`（map）。成员通过关联表管理，角色为 `role`（lead | member）。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/teams` | 已认证 | 列表（分页） |
| GET | `/teams/:id` | 已认证 | 按 ID 获取 |
| GET | `/teams/:id/members` | 已认证 | 列出团队成员 |
| POST | `/teams` | 管理权限 | 创建 |
| PUT | `/teams/:id` | 管理权限 | 更新 |
| DELETE | `/teams/:id` | 管理权限 | 删除 |
| POST | `/teams/:id/members` | 管理权限 | 添加成员 |
| DELETE | `/teams/:id/members/:uid` | 管理权限 | 移除成员 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 |
|------|------|------|
| `name` | string | 是 |
| `description` | string | 否 |
| `labels` | map[string]string | 否 |

**添加成员请求体：**

```json
{ "user_id": 5, "role": "lead" }
```

角色可选 `"lead"` 或 `"member"`。

---

## 18. 值班管理

值班排班管理，支持轮转、班次、替班和参与人。

### 排班 CRUD

**模型字段：** `name`、`team_id`、`description`、`rotation_type`（daily | weekly | custom）、`timezone`、`handoff_time`、`handoff_day`、`is_enabled`、`severity_filter`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/schedules` | 已认证 | 列表（分页）。筛选：`?team_id=1` |
| GET | `/schedules/:id` | 已认证 | 按 ID 获取 |
| GET | `/schedules/:id/oncall` | 已认证 | 获取当前值班用户 |
| POST | `/schedules` | 管理权限 | 创建 |
| PUT | `/schedules/:id` | 管理权限 | 更新 |
| DELETE | `/schedules/:id` | 管理权限 | 删除 |
| PUT | `/schedules/:id/participants` | 管理权限 | 设置轮转参与人 |
| POST | `/schedules/:id/overrides` | 管理权限 | 创建替班 |
| DELETE | `/schedules/:id/overrides/:oid` | 管理权限 | 删除替班 |

**创建 / 更新请求体：**

| 字段 | 类型 | 必填 | 默认值 |
|------|------|------|--------|
| `name` | string | 是 | |
| `team_id` | uint | 否 | |
| `description` | string | 否 | |
| `rotation_type` | string | 是 | |
| `timezone` | string | 否 | `"Asia/Shanghai"` |
| `handoff_time` | string | 否 | `"09:00"` |
| `handoff_day` | int | 否 | |
| `is_enabled` | bool | 否 | true |

**设置参与人请求体：**

```json
{ "user_ids": [1, 2, 3] }
```

**创建替班请求体：**

```json
{
  "user_id": 5,
  "start_time": "2026-04-05T00:00:00Z",
  "end_time": "2026-04-06T00:00:00Z",
  "reason": "Coverage swap"
}
```

### 值班班次

**模型字段：** `schedule_id`、`user_id`、`start_time`、`end_time`、`severity_filter`、`source`（manual | rotation）、`note`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/schedules/:id/shifts` | 已认证 | 列出班次。筛选：`?start=<RFC3339>&end=<RFC3339>` |
| POST | `/schedules/:id/shifts` | 管理权限 | 创建班次 |
| PUT | `/schedules/:id/shifts/:shiftId` | 管理权限 | 更新班次 |
| DELETE | `/schedules/:id/shifts/:shiftId` | 管理权限 | 删除班次 |
| POST | `/schedules/:id/generate-shifts` | 管理权限 | 根据轮转自动生成班次 |

**创建 / 更新班次请求体：**

| 字段 | 类型 | 必填 |
|------|------|------|
| `user_id` | uint | 是 |
| `start_time` | datetime (RFC 3339) | 是 |
| `end_time` | datetime (RFC 3339) | 是 |
| `severity_filter` | string | 否 |
| `note` | string | 否 |

**自动生成班次请求体：**

```json
{ "weeks": 4 }
```

校验范围：1–52 周。

**生成响应：**

```json
{ "code": 0, "data": { "message": "shifts generated", "weeks": 4 } }
```

---

## 19. 升级策略

多步骤升级策略，定义通知目标和延迟间隔。

### 策略 CRUD

**模型字段：** `name`、`team_id`、`is_enabled`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| GET | `/escalation-policies` | 已认证 | 列表。筛选：`?team_id=1` |
| GET | `/escalation-policies/:id` | 已认证 | 获取策略及其步骤 |
| POST | `/escalation-policies` | 管理权限 | 创建 |
| PUT | `/escalation-policies/:id` | 管理权限 | 更新 |
| DELETE | `/escalation-policies/:id` | 管理权限 | 删除 |

**创建 / 更新请求体：**

```json
{ "name": "Critical P1", "team_id": 1, "is_enabled": true }
```

**获取响应：**

```json
{
  "code": 0,
  "data": {
    "policy": { "id": 1, "name": "Critical P1", "team_id": 1, "is_enabled": true },
    "steps": [ { "id": 1, "step_order": 1, "delay_minutes": 0, "target_type": "schedule", "target_id": 1 }, ... ]
  }
}
```

### 升级步骤

**模型字段：** `policy_id`、`step_order`、`delay_minutes`、`target_type`（user | schedule | team）、`target_id`、`notify_channel_id`。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| POST | `/escalation-policies/:id/steps` | 管理权限 | 创建步骤 |
| PUT | `/escalation-policies/:id/steps/:stepId` | 管理权限 | 更新步骤 |
| DELETE | `/escalation-policies/:id/steps/:stepId` | 管理权限 | 删除步骤 |

**创建 / 更新步骤请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `step_order` | int | 是 | 执行顺序（从 1 开始） |
| `delay_minutes` | int | 是 | 触发前的延迟时间（分钟） |
| `target_type` | string | 是 | user、schedule 或 team |
| `target_id` | uint | 是 | 关联目标实体 ID |
| `notify_channel_id` | uint | 否 | 覆盖默认通知渠道 |

---

## 20. AI

AI 驱动的告警分析。支持 LLM 生成的告警报告和 SOP 建议。

| 方法 | 路由 | 访问级别 | 说明 |
|------|------|----------|------|
| POST | `/ai/alert-report` | 已认证 | 生成 AI 告警分析报告 |
| POST | `/ai/suggest-sop` | 已认证 | AI 推荐的告警 SOP |
| POST | `/ai/test` | 管理权限 | 测试 AI 提供商连通性 |
| GET | `/ai/config` | 仅管理员 | 获取 AI 配置（API key 已脱敏） |
| PUT | `/ai/config` | 仅管理员 | 更新 AI 配置 |

**生成报告 / 推荐 SOP 请求体：**

```json
{ "event_id": 42 }
```

**报告响应：**

```json
{ "code": 0, "data": { "report": "## Analysis\n...", "event_id": 42 } }
```

**SOP 响应：**

```json
{ "code": 0, "data": { "sop": "1. Check CPU usage\n2. ...", "event_id": 42 } }
```

**测试响应：**

```json
{ "code": 0, "data": { "message": "AI connection successful" } }
```

---

## 21. 飞书机器人

飞书（Lark）机器人集成，用于交互式告警通知。

### POST `/lark/event` — 飞书事件回调

**访问级别：** 公开（通过飞书验证令牌校验）

接收飞书事件订阅回调，包括 URL 验证挑战和消息事件。返回原始 JSON 以兼容飞书协议。

### GET `/api/v1/lark/bot/config` — 获取飞书配置

**访问级别：** 仅管理员

返回飞书机器人配置（App ID、Webhook URL 等）。

### PUT `/api/v1/lark/bot/config` — 更新飞书配置

**访问级别：** 仅管理员

更新飞书机器人配置。敏感字段（app_secret、verification_token、encrypt_key）使用 AES-GCM 加密存储。

---

## 22. 引擎

### GET `/api/v1/engine/status` — 引擎状态

**访问级别：** 已认证

返回告警评估引擎状态，包括活跃规则数量、评估指标和状态存储连通性。

---

## 23. 仪表盘

### GET `/api/v1/dashboard/stats` — 仪表盘统计

**访问级别：** 已认证

**响应：**

```json
{
  "code": 0,
  "data": {
    "total_datasources": 3,
    "total_rules": 45,
    "active_alerts": 12,
    "resolved_today": 8,
    "total_users": 20,
    "total_teams": 4
  }
}
```

---

## 24. Webhook

### POST `/webhooks/alertmanager` — Alertmanager Webhook

**访问级别：** 公开（通过共享密钥或网络层面的源 IP 进行认证）

接收 [Alertmanager webhook 格式](https://prometheus.io/docs/alerting/latest/configuration/#webhook_config) 的告警负载。

**请求体：**

```json
{
  "version": "4",
  "status": "firing",
  "receiver": "sreagent",
  "alerts": [
    {
      "status": "firing",
      "labels": { "alertname": "HighCPU", "severity": "critical", "instance": "node1:9090" },
      "annotations": { "summary": "CPU usage above 90%", "description": "..." },
      "startsAt": "2026-04-04T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus:9090/graph?...",
      "fingerprint": "abc123"
    }
  ],
  "groupLabels": { "alertname": "HighCPU" },
  "commonLabels": { "alertname": "HighCPU", "severity": "critical" },
  "commonAnnotations": { "summary": "CPU usage above 90%" },
  "externalURL": "http://alertmanager:9093"
}
```

---

## 25. 告警操作页面

通过令牌认证的 HTML 页面，从飞书通知卡片中链接。允许一键执行告警操作，无需访问完整 UI。

### GET `/alert-action/:token` — 渲染操作页面

**访问级别：** URL 路径中的令牌（JWT）

| 查询参数 | 说明 |
|----------|------|
| `action` | 预选操作：acknowledge、silence、resolve、close |
| `duration` | 预填静默时长（分钟） |

返回包含操作表单的 HTML 页面。

### POST `/alert-action/:token` — 执行操作

**访问级别：** URL 路径中的令牌（JWT）

**表单字段**（`application/x-www-form-urlencoded`）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `action` | string | acknowledge、silence、resolve、close |
| `operator_name` | string | 操作人 |
| `note` | string | 备注（可选） |
| `duration` | string | 静默时长（分钟），仅用于 silence 操作 |

返回 HTML 结果页面（成功或错误）。

---

## 路由汇总

| 类别 | 数量 | 访问级别 |
|------|------|----------|
| 公开（无需认证） | 10 | 健康检查、登录、OIDC、Webhook、飞书回调、操作页面 |
| 只读（已认证） | 36 | 所有 GET/列表端点 |
| 操作权限（member 及以上） | 12 | 告警操作、订阅规则 |
| 管理权限（team_lead 及以上） | 35 | 配置 CRUD、渠道、规则、排班、团队 |
| 仅管理员 | 10 | 用户 CRUD、系统设置、AI/飞书配置 |
| **合计** | **~87** | |
