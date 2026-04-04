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
  MuteRule,
  NotifyRule,
  NotifyMedia,
  MessageTemplate,
  SubscribeRule,
  BizGroup,
  EngineStatus,
  AlertChannel,
  UserNotifyConfig,
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
    request.post<ApiResponse<{ status: string }>>(`/datasources/${id}/health-check`),
}

// ===== Alert Rule API =====
export const alertRuleApi = {
  list: (params?: { page?: number; page_size?: number; severity?: string; status?: string; group_name?: string }) =>
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
