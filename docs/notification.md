# 通知管道 (Notification Pipeline)

## 完整链路

```
Engine fires event
  → SetOnAlert callback (main.go)
    → AlertGroupManager.ProcessEvent()     ← group_wait/interval 缓冲
      → InhibitionRuleService.IsInhibited() ← 告警抑制
      → MuteRuleService.IsAlertMuted()     ← 静默规则
      → NotificationService.RouteAlert()
        → v1 策略管道 (NotifyChannel + NotifyPolicy)
        → v2 规则管道 (NotifyRule with Pipeline)
        → processSubscriptions()
          → SubscribeRuleService.FindSubscriptions()
          → NotifyRuleService.ProcessEvent()
            → SendNotification()
```

## 通知渠道类型

| 类型 | 配置格式 | 说明 |
|------|----------|------|
| `lark_webhook` | `{"webhook_url": "https://..."}` | 飞书 Incoming Webhook |
| `lark_bot` | `{"webhook_url": "..."}` (或 Bot API 自动) | 飞书机器人，支持 DM |
| `email` | SMTP 配置 (host/port/tls/username/password/recipients) | 系统级 SMTP (system_settings group=smtp) |
| `custom_webhook` | `{"url": "...", "method": "POST", "headers": {}, "timeout_seconds": 10}` | 自定义 HTTP Webhook |
| `script` | `{"command": "bash /path/to/script.sh"}` | 脚本执行 |

## v1 策略管道

```
NotifyChannel（渠道配置）
  → NotifyPolicy（匹配 labels → 渠道绑定）
    → NotifyRecord（发送记录）
```

## v2 规则管道

```
NotifyRule（匹配 labels + pipeline steps）
  → PipelineStep（条件过滤、转换）
  → NotifyConfig（目标 media）
  → NotifyMedia（具体渠道配置）
```

## 订阅机制

- `SubscribeRule` 按 labels 匹配告警事件
- 命中的订阅规则 → 额外接收人通过 NotifyRule 管道发送
- 用于：团队订阅、个人订阅特定业务告警

## 告警抑制 (Inhibition)

- Alertmanager 风格：source_match → target_match + equal_labels
- 当源告警 firing 时，目标告警被抑制
- 场景：主机宕机 → 屏蔽其上所有服务告警

## 静默规则 (Mute)

- 时间窗口 + 周期性静默（days_of_week）
- 支持 preview API 预览命中效果
- 在管道最前端检查，命中则跳过所有通知

## Lark 卡片

- Go template 渲染（`message_template.go`）
- 支持 text / html / markdown / lark_card 四种类型
- 告警恢复时 PATCH 更新卡片状态
- Bot 指令：`/ack`, `/resolve`, `/assign`, `/comment`
