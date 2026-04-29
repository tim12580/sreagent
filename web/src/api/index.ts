import request from './request'
import type {
  ApiResponse,
  PageData,
  LoginRequest,
  LoginResponse,
  User,
  DataSource,
  AlertRule,
  AlertEvent,
  AlertEventFilter,
  AlertTimeline,
  Team,
  Schedule,
  ScheduleParticipant,
  ScheduleOverride,
  OnCallShift,
  EscalationPolicy,
  EscalationStep,
  DashboardStats,
  MTTRStats,
  MTTRTrendPoint,
  MuteRule,
  NotifyRule,
  NotifyMedia,
  MessageTemplate,
  SubscribeRule,
  BizGroup,
  EngineStatus,
  AlertChannel,
  UserNotifyConfig,
  AuditLog,
  AlertTrendPoint,
  TopRuleItem,
  SeverityHistoryPoint,
  QueryResponse,
  AlertGroupItem,
  InhibitionRule,
  LogEntry,
} from '@/types'

// ===== Auth API =====
export const authApi = {
  login: (data: LoginRequest) =>
    request.post<ApiResponse<LoginResponse>>('/auth/login', data),

  getProfile: () =>
    request.get<ApiResponse<User>>('/auth/profile'),

  updateMe: (data: { display_name?: string; email?: string; phone?: string; avatar?: string }) =>
    request.put<ApiResponse<null>>('/me/profile', data),

  changeMyPassword: (data: { old_password: string; new_password: string }) =>
    request.post<ApiResponse<null>>('/me/password', data),

  /** Refresh an existing JWT token (may be recently expired, within 7-day grace window) */
  refreshToken: (token: string) =>
    request.post<ApiResponse<LoginResponse>>('/auth/refresh', { token }),

  /** Bind (or clear) the current user's Lark open_id for bot identity mapping */
  bindLark: (larkOpenId: string) =>
    request.put<ApiResponse<null>>('/me/lark-bind', { lark_open_id: larkOpenId }),

  /** Check if OIDC SSO is enabled and get the login URL */
  getOIDCConfig: () =>
    request.get<ApiResponse<{ enabled: boolean; login_url?: string }>>('/auth/oidc/config'),
}

// ===== DataSource API =====
export const datasourceApi = {
  list: (params?: { page?: number; page_size?: number; type?: string }) =>
    request.get<ApiResponse<PageData<DataSource>>>('/datasources', { params }),

  get: (id: number) =>
    request.get<ApiResponse<DataSource>>(`/datasources/${id}`),

  create: (data: Partial<DataSource>) =>
    request.post<ApiResponse<DataSource>>('/datasources', data),

  update: (id: number, data: Partial<DataSource>) =>
    request.put<ApiResponse<DataSource>>(`/datasources/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/datasources/${id}`),

  healthCheck: (id: number) =>
    request.post<ApiResponse<{ status: string; message: string; latency_ms: number; version: string }>>(`/datasources/${id}/health-check`),

  query: (id: number, data: { expression: string; time?: number }) =>
    request.post<ApiResponse<QueryResponse>>(`/datasources/${id}/query`, data),

  rangeQuery: (id: number, data: { expression: string; start: number; end: number; step: string }) =>
    request.post<ApiResponse<QueryResponse>>(`/datasources/${id}/query-range`, data),

  labelKeys: (id: number) =>
    request.get<ApiResponse<string[]>>(`/datasources/${id}/labels/keys`),

  labelValues: (id: number, key: string) =>
    request.get<ApiResponse<string[]>>(`/datasources/${id}/labels/values`, { params: { key } }),

  metricNames: (id: number, search?: string, limit = 100) =>
    request.get<ApiResponse<string[]>>(`/datasources/${id}/metrics`, { params: { search, limit } }),

  logQuery: (id: number, data: { expression: string; start: number; end: number; limit?: number }) =>
    request.post<ApiResponse<{ entries: LogEntry[]; total: number; truncated: boolean }>>(`/datasources/${id}/log-query`, data),
}

// ===== Alert Rule API =====
export const alertRuleApi = {
  list: (params?: { page?: number; page_size?: number; severity?: string; status?: string; group_name?: string; category?: string }) =>
    request.get<ApiResponse<PageData<AlertRule>>>('/alert-rules', { params }),

  get: (id: number) =>
    request.get<ApiResponse<AlertRule>>(`/alert-rules/${id}`),

  create: (data: Partial<AlertRule>) =>
    request.post<ApiResponse<AlertRule>>('/alert-rules', data),

  update: (id: number, data: Partial<AlertRule>) =>
    request.put<ApiResponse<AlertRule>>(`/alert-rules/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/alert-rules/${id}`),

  toggleStatus: (id: number, status: string) =>
    request.patch<ApiResponse<null>>(`/alert-rules/${id}/status`, { status }),

  listCategories: () =>
    request.get<ApiResponse<string[]>>('/alert-rules/categories'),

  exportRules: (params?: { format?: string; category?: string; group_name?: string }) =>
    request.get('/alert-rules/export', { params, responseType: 'blob' }),

  importRules: (file: File, datasourceId?: number) => {
    const formData = new FormData()
    formData.append('file', file)
    if (datasourceId) formData.append('datasource_id', String(datasourceId))
    return request.post<ApiResponse<{ total: number; success: number; failed: number; errors: string[] }>>('/alert-rules/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
}

// ===== Alert Event API =====
export const alertEventApi = {
  list: (params?: AlertEventFilter) =>
    request.get<ApiResponse<PageData<AlertEvent>>>('/alert-events', { params }),

  get: (id: number) =>
    request.get<ApiResponse<AlertEvent>>(`/alert-events/${id}`),

  acknowledge: (id: number) =>
    request.post<ApiResponse<null>>(`/alert-events/${id}/acknowledge`),

  assign: (id: number, data: { assign_to: number; note?: string }) =>
    request.post<ApiResponse<null>>(`/alert-events/${id}/assign`, data),

  resolve: (id: number, data?: { resolution?: string }) =>
    request.post<ApiResponse<null>>(`/alert-events/${id}/resolve`, data),

  close: (id: number, data?: { note?: string }) =>
    request.post<ApiResponse<null>>(`/alert-events/${id}/close`, data),

  silence: (id: number, data: { duration_minutes: number; reason: string }) =>
    request.post<ApiResponse<null>>(`/alert-events/${id}/silence`, data),

  comment: (id: number, data: { note: string }) =>
    request.post<ApiResponse<null>>(`/alert-events/${id}/comment`, data),

  getTimeline: (id: number) =>
    request.get<ApiResponse<AlertTimeline[]>>(`/alert-events/${id}/timeline`),

  batchAcknowledge: (ids: number[]) =>
    request.post<ApiResponse<null>>('/alert-events/batch/acknowledge', { ids }),

  batchClose: (ids: number[]) =>
    request.post<ApiResponse<null>>('/alert-events/batch/close', { ids }),
}

// ===== User API =====
export const userApi = {
  list: (params?: { page?: number; page_size?: number; role?: string; is_active?: boolean }) =>
    request.get<ApiResponse<PageData<User>>>('/users', { params }),

  get: (id: number) =>
    request.get<ApiResponse<User>>(`/users/${id}`),

  create: (data: Partial<User> & { password?: string }) =>
    request.post<ApiResponse<User>>('/users', data),

  update: (id: number, data: Partial<User>) =>
    request.put<ApiResponse<User>>(`/users/${id}`, data),

  toggleActive: (id: number, is_active: boolean) =>
    request.patch<ApiResponse<null>>(`/users/${id}/active`, { is_active }),

  changePassword: (id: number, data: { password: string }) =>
    request.patch<ApiResponse<null>>(`/users/${id}/password`, data),

  createVirtual: (data: { username: string; display_name: string; user_type: 'bot' | 'channel'; notify_target?: string }) =>
    request.post<ApiResponse<User>>('/users/virtual', data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/users/${id}`),
}

// ===== Team API =====
export const teamApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<Team>>>('/teams', { params }),

  get: (id: number) =>
    request.get<ApiResponse<Team>>(`/teams/${id}`),

  create: (data: Partial<Team>) =>
    request.post<ApiResponse<Team>>('/teams', data),

  update: (id: number, data: Partial<Team>) =>
    request.put<ApiResponse<Team>>(`/teams/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/teams/${id}`),

  addMember: (teamId: number, userId: number) =>
    request.post<ApiResponse<null>>(`/teams/${teamId}/members`, { user_id: userId }),

  removeMember: (teamId: number, userId: number) =>
    request.delete<ApiResponse<null>>(`/teams/${teamId}/members/${userId}`),

  listMembers: (teamId: number) =>
    request.get<ApiResponse<User[]>>(`/teams/${teamId}/members`),
}

// ===== Schedule API =====
export const scheduleApi = {
  list: (params?: { page?: number; page_size?: number; team_id?: number }) =>
    request.get<ApiResponse<PageData<Schedule>>>('/schedules', { params }),

  get: (id: number) =>
    request.get<ApiResponse<Schedule>>(`/schedules/${id}`),

  create: (data: Partial<Schedule>) =>
    request.post<ApiResponse<Schedule>>('/schedules', data),

  update: (id: number, data: Partial<Schedule>) =>
    request.put<ApiResponse<Schedule>>(`/schedules/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/schedules/${id}`),

  getCurrentOnCall: (id: number) =>
    request.get<ApiResponse<User | null>>(`/schedules/${id}/oncall`),

  setParticipants: (id: number, participants: { user_id: number; position: number }[]) =>
    request.put<ApiResponse<ScheduleParticipant[]>>(`/schedules/${id}/participants`, { participants }),

  getParticipants: (id: number) =>
    request.get<ApiResponse<ScheduleParticipant[]>>(`/schedules/${id}/participants`),

  createOverride: (id: number, data: { user_id: number; start_time: string; end_time: string; reason: string }) =>
    request.post<ApiResponse<ScheduleOverride>>(`/schedules/${id}/overrides`, data),

  listOverrides: (id: number) =>
    request.get<ApiResponse<ScheduleOverride[]>>(`/schedules/${id}/overrides`),

  deleteOverride: (id: number, overrideId: number) =>
    request.delete<ApiResponse<null>>(`/schedules/${id}/overrides/${overrideId}`),

  listShifts: (id: number, params: { start?: string; end?: string }) =>
    request.get<ApiResponse<OnCallShift[]>>(`/schedules/${id}/shifts`, { params }),

  createShift: (id: number, data: Partial<OnCallShift>) =>
    request.post<ApiResponse<OnCallShift>>(`/schedules/${id}/shifts`, data),

  updateShift: (id: number, shiftId: number, data: Partial<OnCallShift>) =>
    request.put<ApiResponse<OnCallShift>>(`/schedules/${id}/shifts/${shiftId}`, data),

  deleteShift: (id: number, shiftId: number) =>
    request.delete<ApiResponse<null>>(`/schedules/${id}/shifts/${shiftId}`),

  generateShifts: (id: number, data: { weeks: number }) =>
    request.post<ApiResponse<null>>(`/schedules/${id}/generate-shifts`, data),
}

// ===== Escalation Policy API =====
export const escalationApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<EscalationPolicy>>>('/escalation-policies', { params }),

  get: (id: number) =>
    request.get<ApiResponse<EscalationPolicy>>(`/escalation-policies/${id}`),

  create: (data: Partial<EscalationPolicy>) =>
    request.post<ApiResponse<EscalationPolicy>>('/escalation-policies', data),

  update: (id: number, data: Partial<EscalationPolicy>) =>
    request.put<ApiResponse<EscalationPolicy>>(`/escalation-policies/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/escalation-policies/${id}`),

  createStep: (policyId: number, data: Partial<EscalationStep>) =>
    request.post<ApiResponse<EscalationStep>>(`/escalation-policies/${policyId}/steps`, data),

  updateStep: (policyId: number, stepId: number, data: Partial<EscalationStep>) =>
    request.put<ApiResponse<EscalationStep>>(`/escalation-policies/${policyId}/steps/${stepId}`, data),

  deleteStep: (policyId: number, stepId: number) =>
    request.delete<ApiResponse<null>>(`/escalation-policies/${policyId}/steps/${stepId}`),
}

// ===== Mute Rule API =====
export const muteRuleApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<MuteRule>>>('/mute-rules', { params }),

  get: (id: number) =>
    request.get<ApiResponse<MuteRule>>(`/mute-rules/${id}`),

  create: (data: Partial<MuteRule>) =>
    request.post<ApiResponse<MuteRule>>('/mute-rules', data),

  update: (id: number, data: Partial<MuteRule>) =>
    request.put<ApiResponse<MuteRule>>(`/mute-rules/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/mute-rules/${id}`),

  preview: () =>
    request.get<ApiResponse<Array<{
      rule_id: number; rule_name: string
      matched_count: number; matched_alerts: AlertEvent[]
    }>>>('/mute-rules/preview'),
}

// ===== Notify Rule API (v2) =====
export const notifyRuleApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<NotifyRule>>>('/notify-rules', { params }),

  get: (id: number) =>
    request.get<ApiResponse<NotifyRule>>(`/notify-rules/${id}`),

  create: (data: Partial<NotifyRule>) =>
    request.post<ApiResponse<NotifyRule>>('/notify-rules', data),

  update: (id: number, data: Partial<NotifyRule>) =>
    request.put<ApiResponse<NotifyRule>>(`/notify-rules/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/notify-rules/${id}`),
}

// ===== Notify Media API =====
export const notifyMediaApi = {
  list: (params?: { page?: number; page_size?: number; type?: string }) =>
    request.get<ApiResponse<PageData<NotifyMedia>>>('/notify-media', { params }),

  get: (id: number) =>
    request.get<ApiResponse<NotifyMedia>>(`/notify-media/${id}`),

  create: (data: Partial<NotifyMedia>) =>
    request.post<ApiResponse<NotifyMedia>>('/notify-media', data),

  update: (id: number, data: Partial<NotifyMedia>) =>
    request.put<ApiResponse<NotifyMedia>>(`/notify-media/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/notify-media/${id}`),

  test: (id: number) =>
    request.post<ApiResponse<{ success: boolean; message: string }>>(`/notify-media/${id}/test`),
}

// ===== Message Template API =====
export const messageTemplateApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<MessageTemplate>>>('/message-templates', { params }),

  get: (id: number) =>
    request.get<ApiResponse<MessageTemplate>>(`/message-templates/${id}`),

  create: (data: Partial<MessageTemplate>) =>
    request.post<ApiResponse<MessageTemplate>>('/message-templates', data),

  update: (id: number, data: Partial<MessageTemplate>) =>
    request.put<ApiResponse<MessageTemplate>>(`/message-templates/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/message-templates/${id}`),

  preview: (data: { content: string; type: string }) =>
    request.post<ApiResponse<{ rendered: string }>>('/message-templates/preview', data),
}

// ===== Subscribe Rule API =====
export const subscribeRuleApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<SubscribeRule>>>('/subscribe-rules', { params }),

  get: (id: number) =>
    request.get<ApiResponse<SubscribeRule>>(`/subscribe-rules/${id}`),

  create: (data: Partial<SubscribeRule>) =>
    request.post<ApiResponse<SubscribeRule>>('/subscribe-rules', data),

  update: (id: number, data: Partial<SubscribeRule>) =>
    request.put<ApiResponse<SubscribeRule>>(`/subscribe-rules/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/subscribe-rules/${id}`),
}

// ===== Business Group API =====
export const bizGroupApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<BizGroup>>>('/biz-groups', { params }),

  tree: () =>
    request.get<ApiResponse<BizGroup[]>>('/biz-groups/tree'),

  get: (id: number) =>
    request.get<ApiResponse<BizGroup>>(`/biz-groups/${id}`),

  create: (data: Partial<BizGroup>) =>
    request.post<ApiResponse<BizGroup>>('/biz-groups', data),

  update: (id: number, data: Partial<BizGroup>) =>
    request.put<ApiResponse<BizGroup>>(`/biz-groups/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/biz-groups/${id}`),

  addMember: (id: number, data: { user_id: number; role?: string }) =>
    request.post<ApiResponse<null>>(`/biz-groups/${id}/members`, data),

  removeMember: (id: number, uid: number) =>
    request.delete<ApiResponse<null>>(`/biz-groups/${id}/members/${uid}`),

  listMembers: (id: number) =>
    request.get<ApiResponse<User[]>>(`/biz-groups/${id}/members`),
}

// ===== Engine API =====
export const engineApi = {
  getStatus: () =>
    request.get<ApiResponse<EngineStatus>>('/engine/status'),
}

// ===== Alert Channel API =====
export const alertChannelApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<AlertChannel>>>('/alert-channels', { params }),

  get: (id: number) =>
    request.get<ApiResponse<AlertChannel>>(`/alert-channels/${id}`),

  create: (data: Partial<AlertChannel>) =>
    request.post<ApiResponse<AlertChannel>>('/alert-channels', data),

  update: (id: number, data: Partial<AlertChannel>) =>
    request.put<ApiResponse<AlertChannel>>(`/alert-channels/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/alert-channels/${id}`),

  test: (id: number) =>
    request.post<ApiResponse<{ success: boolean; message: string }>>(`/alert-channels/${id}/test`),
}

// ===== User Notify Config API =====
export const userNotifyConfigApi = {
  // Returns array of all configs for current user
  list: () => request.get<ApiResponse<UserNotifyConfig[]>>('/me/notify-configs'),
  // Upsert one config (keyed by media_type)
  upsert: (data: Partial<UserNotifyConfig>) => request.put<ApiResponse<UserNotifyConfig>>('/me/notify-configs', data),
  // Delete by media type
  deleteByType: (mediaType: string) => request.delete<ApiResponse<null>>(`/me/notify-configs/${mediaType}`),
}

// ===== Dashboard API =====
export const dashboardApi = {
  getStats: () =>
    request.get<ApiResponse<DashboardStats>>('/dashboard/stats'),
  getMTTRStats: (hours = 24) =>
    request.get<ApiResponse<MTTRStats>>('/dashboard/mtta-mttr', { params: { hours } }),
  getMTTRTrend: (days = 30) =>
    request.get<ApiResponse<MTTRTrendPoint[]>>('/dashboard/mttr-trend', { params: { days } }),
  getAlertTrend: (days = 30) =>
    request.get<ApiResponse<AlertTrendPoint[]>>('/dashboard/alert-trend', { params: { days } }),
  getTopRules: (days = 30, limit = 10) =>
    request.get<ApiResponse<TopRuleItem[]>>('/dashboard/top-rules', { params: { days, limit } }),
  getSeverityHistory: (days = 30) =>
    request.get<ApiResponse<SeverityHistoryPoint[]>>('/dashboard/severity-history', { params: { days } }),
  exportReportURL: (startDate: string, endDate: string) =>
    `/api/v1/dashboard/export?start_date=${startDate}&end_date=${endDate}`,
}

// ===== AI API =====
export const aiApi = {
  getConfig: () =>
    request.get<ApiResponse<{ provider: string; api_key: string; base_url: string; model: string; enabled: boolean }>>('/ai/config'),

  updateConfig: (data: { provider?: string; api_key?: string; base_url?: string; model?: string; enabled?: boolean }) =>
    request.put<ApiResponse<null>>('/ai/config', data),

  testConnection: () =>
    request.post<ApiResponse<{ success: boolean; message: string }>>('/ai/test'),

  generateReport: (eventId: number) =>
    request.post<ApiResponse<{ summary: string; probable_causes: string[]; impact: string; recommended_steps: string[] }>>('/ai/alert-report', { event_id: eventId }),

  suggestSOP: (eventId: number) =>
    request.post<ApiResponse<{ title: string; steps: string[]; references: string[] }>>('/ai/suggest-sop', { event_id: eventId }),
}

// ===== Lark Bot API =====
export const larkBotApi = {
  getConfig: () =>
    request.get<ApiResponse<{ app_id: string; app_secret: string; default_webhook: string; verification_token: string; encrypt_key: string; bot_enabled: boolean }>>('/lark/bot/config'),

  updateConfig: (data: { app_id?: string; app_secret?: string; default_webhook?: string; verification_token?: string; encrypt_key?: string; bot_enabled?: boolean }) =>
    request.put<ApiResponse<null>>('/lark/bot/config', data),
}

// ===== Audit Log API =====
export const auditLogApi = {
  list: (params?: {
    page?: number; page_size?: number;
    action?: string; resource_type?: string;
    start_time?: string; end_time?: string;
  }) => request.get<ApiResponse<PageData<AuditLog>>>('/audit-logs', { params }),
}

// ===== OIDC Settings API =====
export const oidcSettingsApi = {
  getConfig: () =>
    request.get<ApiResponse<{
      enabled: boolean
      issuer_url: string
      client_id: string
      client_secret: string
      redirect_url: string
      scopes: string
      role_claim: string
      role_mapping: string
      default_role: string
      auto_provision: boolean
    }>>('/settings/oidc'),

  updateConfig: (data: {
    enabled?: boolean
    issuer_url?: string
    client_id?: string
    client_secret?: string
    redirect_url?: string
    scopes?: string
    role_claim?: string
    role_mapping?: string
    default_role?: string
    auto_provision?: boolean
  }) =>
    request.put<ApiResponse<{ message: string }>>('/settings/oidc', data),
}

// ===== SMTP Settings API =====
export const smtpSettingsApi = {
  getConfig: () =>
    request.get<ApiResponse<{
      smtp_host: string; smtp_port: number; smtp_tls: boolean
      username: string; password: string; from: string; enabled: boolean
    }>>('/settings/smtp'),

  updateConfig: (data: {
    smtp_host?: string; smtp_port?: number; smtp_tls?: boolean
    username?: string; password?: string; from?: string; enabled?: boolean
  }) => request.put<ApiResponse<null>>('/settings/smtp', data),

  testConnection: (to: string) =>
    request.post<ApiResponse<{ message: string }>>('/settings/smtp/test', { to }),
}

// ===== Security Settings API =====
export const securitySettingsApi = {
  getConfig: () =>
    request.get<ApiResponse<{ jwt_expire_seconds: number }>>('/settings/security'),

  updateConfig: (data: { jwt_expire_seconds: number }) =>
    request.put<ApiResponse<null>>('/settings/security', data),
}

// ===== Mute Rule Preview API =====
export const mutePreviewApi = {
  preview: () =>
    request.get<ApiResponse<Array<{
      rule_id: number; rule_name: string
      matched_count: number; matched_alerts: import('@/types').AlertEvent[]
    }>>>('/mute-rules/preview'),
}

// ===== Alert Groups API =====
export const alertGroupsApi = {
  list: (params?: { status?: string; severity?: string }) =>
    request.get<ApiResponse<AlertGroupItem[]>>('/alert-events/groups', { params }),
}

// ===== Alert Export API =====
export const alertExportApi = {
  exportCSV: (params?: {
    status?: string; severity?: string; view_mode?: string
    start?: string; end?: string
  }) => {
    const query = new URLSearchParams()
    if (params?.status) query.set('status', params.status)
    if (params?.severity) query.set('severity', params.severity)
    if (params?.view_mode) query.set('view_mode', params.view_mode)
    if (params?.start) query.set('start', params.start)
    if (params?.end) query.set('end', params.end)
    return `/api/v1/alert-events/export?${query.toString()}`
  },
}

// ===== Inhibition Rules API =====
export const inhibitionRuleApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<InhibitionRule>>>('/inhibition-rules', { params }),

  get: (id: number) =>
    request.get<ApiResponse<InhibitionRule>>(`/inhibition-rules/${id}`),

  create: (data: Partial<InhibitionRule>) =>
    request.post<ApiResponse<InhibitionRule>>('/inhibition-rules', data),

  update: (id: number, data: Partial<InhibitionRule>) =>
    request.put<ApiResponse<InhibitionRule>>(`/inhibition-rules/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/inhibition-rules/${id}`),
}

// ===== iCal Schedule Export =====
export const scheduleICalApi = {
  exportURL: (scheduleId: number) =>
    `/api/v1/schedules/${scheduleId}/ical`,
}

// ===== Heartbeat Ping (public, no auth) =====
export const heartbeatApi = {
  ping: (token: string) =>
    request.post<ApiResponse<{ status: string }>>(`/heartbeat/${token}`),
}

// ===== Label Registry API =====
export const labelRegistryApi = {
  getKeys: (datasourceId?: number) =>
    request.get<ApiResponse<string[]>>('/label-registry/keys', {
      params: datasourceId ? { datasource_id: datasourceId } : {}
    }),

  getValues: (key: string, datasourceId?: number) =>
    request.get<ApiResponse<string[]>>('/label-registry/values', {
      params: { key, ...(datasourceId ? { datasource_id: datasourceId } : {}) }
    }),

  sync: () =>
    request.post<ApiResponse<{ message: string }>>('/label-registry/sync'),
}

// ===== Dashboard V2 API =====
export const dashboardV2Api = {
  list: (params?: { page?: number; page_size?: number; search?: string }) =>
    request.get<ApiResponse<PageData<import('@/types/dashboard').DashboardV2>>>('/dashboards', { params }),

  get: (id: number) =>
    request.get<ApiResponse<import('@/types/dashboard').DashboardV2>>(`/dashboards/${id}`),

  create: (data: Partial<import('@/types/dashboard').DashboardV2>) =>
    request.post<ApiResponse<import('@/types/dashboard').DashboardV2>>('/dashboards', data),

  update: (id: number, data: Partial<import('@/types/dashboard').DashboardV2>) =>
    request.put<ApiResponse<import('@/types/dashboard').DashboardV2>>(`/dashboards/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/dashboards/${id}`),
}

// ===== Event Pipeline API =====
export const pipelineApi = {
  list: (params?: { page?: number; page_size?: number; search?: string }) =>
    request.get<ApiResponse<PageData<import('@/types/pipeline').EventPipeline>>>('/event-pipelines', { params }),

  get: (id: number) =>
    request.get<ApiResponse<import('@/types/pipeline').EventPipeline>>(`/event-pipelines/${id}`),

  create: (data: Partial<import('@/types/pipeline').EventPipeline>) =>
    request.post<ApiResponse<import('@/types/pipeline').EventPipeline>>('/event-pipelines', data),

  update: (id: number, data: Partial<import('@/types/pipeline').EventPipeline>) =>
    request.put<ApiResponse<import('@/types/pipeline').EventPipeline>>(`/event-pipelines/${id}`, data),

  delete: (id: number) =>
    request.delete<ApiResponse<null>>(`/event-pipelines/${id}`),

  tryRun: (data: { pipeline_id: number; event: any }) =>
    request.post<ApiResponse<any>>('/event-pipelines/tryrun', data),

  listExecutions: (id: number, params?: { page?: number; page_size?: number }) =>
    request.get<ApiResponse<PageData<import('@/types/pipeline').PipelineExecution>>>(`/event-pipelines/${id}/executions`, { params }),
}
