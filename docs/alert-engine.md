# 告警引擎 (Alert Engine)

## 状态机

```
inactive → pending（for_duration）→ firing → recovery_hold → resolved
                                        └── nodata（数据缺失）
```

- **pending**: 条件满足但未持续够 `for_duration`，不产生事件
- **firing**: 告警触发，创建 AlertEvent + Timeline
- **recovery_hold**: 条件不再满足，等待 `recovery_hold_seconds` 后确认恢复
- **resolved**: 恢复确认，更新事件状态 + PATCH Lark 卡片
- **nodata**: 查询返回空数据，可配置是否触发

## 核心组件

| 文件 | 职责 |
|------|------|
| `engine/evaluator.go` | RuleEvaluator goroutine 池管理，规则同步 |
| `engine/rule_eval.go` | 每规则状态机，查询执行，指纹生成，状态持久化 |
| `engine/state_store.go` | StateStore 接口 + StateEntry 序列化结构 |
| `engine/suppression.go` | LevelSuppressor 严重级别去重 |
| `engine/escalation_executor.go` | 升级策略执行器（SLA 超时自动升级） |
| `engine/heartbeat_checker.go` | 心跳规则检测器 |

## 数据流

```
Evaluator.Start()
  → syncRules()（每 sync_interval 从 DB 拉取规则）
  → startRuleEvaluators()（每规则一个 goroutine）
    → RuleEvaluator.Run()
      → evaluate()
        → executeQuery()（按 datasource.Type 分发：Prom/VM/VLogs/Zabbix）
        → 状态机转换
        → persistState()（Redis Hash）
        → onAlertFn(event)（回调通知管道）
```

## Redis 状态持久化

- Key: `sreagent:state:{ruleID}` (Hash)
- Field: fingerprint (MD5 of label set)
- Value: JSON StateEntry（含 Labels, Annotations, Status, timestamps, EventID）
- Redis 不可用时优雅降级到纯内存状态

## 心跳监控

- `rule_type=heartbeat`，生成唯一 `heartbeat_token`
- 外部系统 POST `/heartbeat/:token` 上报心跳
- 超过 `heartbeat_interval` 未收到 → firing
- 收到心跳 → resolved

## 升级策略

- `EscalationExecutor` 独立 goroutine，每 60s 扫描
- 告警 N 分钟未认领 → 触发升级步骤
- 支持目标：user / team / schedule
- 支持渠道：lark_personal / email / webhook

## 分组通知

- `AlertGroupManager` 在引擎回调和 RouteAlert 之间
- group_wait: 首次通知等待窗口（缓冲同组事件）
- group_interval: 同组后续通知最小间隔
- resolved 事件绕过分组立即发送
- 默认值 0 = 禁用（向后兼容）
