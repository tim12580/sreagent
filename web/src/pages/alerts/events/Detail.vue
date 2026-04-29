<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage, NIcon } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { alertEventApi, userApi, aiApi } from '@/api'
import type { AlertEvent, AlertTimeline, User } from '@/types'
import { formatTime, formatDuration } from '@/utils/format'
import { getSeverityType, getStatusColor, statusTagColor, getStatusLabelKey } from '@/utils/alert'
import {
  FlameOutline,
  CheckmarkCircleOutline,
  PersonOutline,
  CheckmarkDoneOutline,
  CloseCircleOutline,
  ChatbubbleOutline,
  VolumeOffOutline,
  ArrowUpCircleOutline,
  NotificationsOutline,
  WarningOutline,
  InformationCircleOutline,
  CopyOutline,
  ArrowBackOutline,
  TimeOutline,
} from '@vicons/ionicons5'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { t } = useI18n()
const event = ref<AlertEvent | null>(null)
const timeline = ref<AlertTimeline[]>([])
const commentText = ref('')
const loading = ref(false)

const eventId = Number(route.params.id)

// Silence modal
const showSilenceModal = ref(false)
const silenceDuration = ref(60)
const silenceReason = ref('')
const silenceSaving = ref(false)
const silenceDurationOptions = [
  { label: '30m', value: 30 }, { label: '1h', value: 60 },
  { label: '2h', value: 120 }, { label: '6h', value: 360 },
  { label: '12h', value: 720 }, { label: '24h', value: 1440 },
]

// Assign modal
const showAssignModal = ref(false)
const assignUserId = ref<number | null>(null)
const assignNote = ref('')
const assignSaving = ref(false)
const users = ref<User[]>([])
const userOptions = computed(() =>
  users.value.map(u => ({ label: `${u.display_name} (${u.username})`, value: u.id }))
)

// Severity helpers
const severityIconMap: Record<string, any> = {
  critical: FlameOutline,
  warning: WarningOutline,
  info: InformationCircleOutline,
}
const severityBannerClass = computed(() => `banner--${event.value?.severity ?? 'info'}`)
const severityIcon = computed(() => severityIconMap[event.value?.severity ?? 'info'] ?? InformationCircleOutline)

// Duration
const eventDuration = computed(() => {
  if (!event.value) return '-'
  const firedAt = new Date(event.value.fired_at).getTime()
  if (event.value.status === 'resolved' || event.value.status === 'closed') {
    const end = event.value.resolved_at
      ? new Date(event.value.resolved_at).getTime()
      : (event.value.closed_at ? new Date(event.value.closed_at).getTime() : Date.now())
    return formatDuration(Math.floor((end - firedAt) / 1000))
  }
  return formatDuration(Math.floor((Date.now() - firedAt) / 1000))
})

// Lifecycle steps
const lifecycleSteps = computed(() => {
  const ev = event.value
  if (!ev) return []
  const steps = [
    {
      key: 'fired',
      label: t('alert.firedAt'),
      time: ev.fired_at,
      done: true,
      active: ev.status === 'firing',
    },
    {
      key: 'acked',
      label: t('alert.ackedAt'),
      time: ev.acked_at,
      done: !!ev.acked_at,
      active: ev.status === 'acknowledged',
    },
    {
      key: 'assigned',
      label: t('alert.assignedTo'),
      time: ev.acked_at, // use acked_at as proxy; assigned doesn't have its own timestamp
      done: ev.status === 'assigned' || ev.status === 'resolved' || ev.status === 'closed',
      active: ev.status === 'assigned',
    },
    {
      key: 'resolved',
      label: ev.status === 'closed' ? t('alert.closedAt') : t('alert.resolvedAt'),
      time: ev.resolved_at ?? ev.closed_at,
      done: ev.status === 'resolved' || ev.status === 'closed',
      active: ev.status === 'resolved' || ev.status === 'closed',
    },
  ]
  return steps
})

// Action guards
const canAck = computed(() => event.value?.status === 'firing')
const canAssign = computed(() => event.value?.status === 'firing' || event.value?.status === 'acknowledged')
const canSilence = computed(() => event.value?.status === 'firing' || event.value?.status === 'acknowledged')
const canResolve = computed(() => event.value?.status !== 'resolved' && event.value?.status !== 'closed')
const canClose = computed(() => event.value?.status !== 'closed')

// Timeline icon + color per action
function getTimelineIcon(action: string) {
  switch (action) {
    case 'created':      return FlameOutline
    case 'acknowledged': return CheckmarkCircleOutline
    case 'assigned':     return PersonOutline
    case 'resolved':     return CheckmarkDoneOutline
    case 'closed':       return CloseCircleOutline
    case 'commented':    return ChatbubbleOutline
    case 'silenced':     return VolumeOffOutline
    case 'escalated':    return ArrowUpCircleOutline
    case 'notified':     return NotificationsOutline
    default:             return TimeOutline
  }
}

function getTimelineColor(action: string): string {
  switch (action) {
    case 'created':      return '#ef4444'
    case 'acknowledged': return '#f59e0b'
    case 'assigned':     return '#3b82f6'
    case 'resolved':     return '#10b981'
    case 'closed':       return '#666666'
    case 'commented':    return '#a78bfa'
    case 'silenced':     return '#a78bfa'
    case 'escalated':    return '#ef4444'
    case 'notified':     return '#3b82f6'
    default:             return '#888888'
  }
}

function getTimelineLabel(action: string): string {
  const map: Record<string, string> = {
    created:      t('alert.created'),
    acknowledged: t('alert.acknowledged'),
    assigned:     t('alert.assigned'),
    resolved:     t('alert.resolved'),
    closed:       t('common.close'),
    commented:    t('alert.commented'),
    silenced:     t('alert.silenced'),
    escalated:    t('alert.escalated'),
    notified:     t('alert.notified'),
    reopened:     t('alert.reopened'),
  }
  return map[action] ?? action
}

// Copy label value
function copyLabel(key: string, value: string) {
  navigator.clipboard.writeText(value).then(() => {
    message.success(`${key} copied`)
  })
}

// API calls
async function fetchEvent() {
  loading.value = true
  try {
    const { data } = await alertEventApi.get(eventId)
    event.value = data.data
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

async function fetchTimeline() {
  try {
    const { data } = await alertEventApi.getTimeline(eventId)
    timeline.value = data.data || []
  } catch (err: any) {
    message.error(err.message)
  }
}

async function fetchUsers() {
  try {
    const { data } = await userApi.list({ page: 1, page_size: 200, is_active: true })
    users.value = data.data.list || []
  } catch { /* silent */ }
}

async function handleAck() {
  try {
    await alertEventApi.acknowledge(eventId)
    message.success(t('alert.alertAcknowledged'))
    fetchEvent(); fetchTimeline()
  } catch (err: any) { message.error(err.message) }
}

async function handleResolve() {
  try {
    await alertEventApi.resolve(eventId, { resolution: t('alert.manuallyResolved') })
    message.success(t('alert.alertResolved'))
    fetchEvent(); fetchTimeline()
  } catch (err: any) { message.error(err.message) }
}

async function handleClose() {
  try {
    await alertEventApi.close(eventId)
    message.success(t('alert.alertClosed'))
    fetchEvent(); fetchTimeline()
  } catch (err: any) { message.error(err.message) }
}

function openSilenceModal() {
  silenceDuration.value = 60; silenceReason.value = ''; showSilenceModal.value = true
}

async function handleSilence() {
  if (!silenceReason.value.trim()) { message.warning(t('alert.silenceReasonPlaceholder')); return }
  silenceSaving.value = true
  try {
    await alertEventApi.silence(eventId, { duration_minutes: silenceDuration.value, reason: silenceReason.value })
    message.success(t('alert.silenceSuccess'))
    showSilenceModal.value = false; fetchEvent(); fetchTimeline()
  } catch (err: any) { message.error(err.message) }
  finally { silenceSaving.value = false }
}

function openAssignModal() {
  assignUserId.value = null; assignNote.value = ''; showAssignModal.value = true
  if (users.value.length === 0) fetchUsers()
}

async function handleAssign() {
  if (!assignUserId.value) { message.warning(t('alert.selectUser')); return }
  assignSaving.value = true
  try {
    await alertEventApi.assign(eventId, { assign_to: assignUserId.value, note: assignNote.value || undefined })
    message.success(t('alert.assignSuccess'))
    showAssignModal.value = false; fetchEvent(); fetchTimeline()
  } catch (err: any) { message.error(err.message) }
  finally { assignSaving.value = false }
}

async function handleComment() {
  if (!commentText.value.trim()) return
  try {
    await alertEventApi.comment(eventId, { note: commentText.value })
    commentText.value = ''; message.success(t('alert.commentAdded')); fetchTimeline()
  } catch (err: any) { message.error(err.message) }
}

onMounted(() => { fetchEvent(); fetchTimeline() })

// AI Analysis
const aiReport = ref<{ summary: string; probable_causes: string[]; impact: string; recommended_steps: string[] } | null>(null)
const aiReportLoading = ref(false)
const aiReportError = ref('')

async function generateAIReport() {
  aiReportLoading.value = true; aiReportError.value = ''; aiReport.value = null
  try {
    const res = await aiApi.generateReport(eventId)
    aiReport.value = res.data.data ?? null
  } catch (err: any) {
    aiReportError.value = err.message || t('alert.aiReportError')
  } finally { aiReportLoading.value = false }
}
</script>

<template>
  <div class="event-detail" v-if="event">

    <!-- ═══ INCIDENT BANNER ═══ -->
    <div class="incident-banner" :class="severityBannerClass">
      <n-button text class="back-btn" @click="router.back()">
        <template #icon><n-icon :component="ArrowBackOutline" /></template>
        {{ t('alert.backToEvents') }}
      </n-button>

      <div class="banner-body">
        <div class="banner-icon-wrap">
          <n-icon :component="severityIcon" size="36" />
        </div>
        <div class="banner-text">
          <div class="banner-title">{{ event.alert_name }}</div>
          <div class="banner-meta">
            <n-tag :type="getSeverityType(event.severity)" size="small" round style="font-weight:600">
              {{ event.severity.toUpperCase() }}
            </n-tag>
            <n-tag size="small" :bordered="false" :color="statusTagColor(event.status)" style="font-weight:600">
              {{ t(getStatusLabelKey(event.status)) }}
            </n-tag>
            <span class="meta-chip">
              <n-icon :component="TimeOutline" size="12" />
              {{ eventDuration }}
            </span>
            <span class="meta-chip fire-count">
              <n-icon :component="FlameOutline" size="12" />
              ×{{ event.fire_count }}
            </span>
            <span v-if="event.source" class="meta-chip">{{ event.source }}</span>
          </div>
        </div>
        <div class="banner-actions">
          <n-button v-if="canAck" type="primary" size="small" @click="handleAck">
            {{ t('alert.acknowledge') }}
          </n-button>
          <n-button v-if="canAssign" type="info" size="small" @click="openAssignModal">
            {{ t('alert.assign') }}
          </n-button>
          <n-button v-if="canSilence" size="small" secondary @click="openSilenceModal">
            {{ t('alert.silence') }}
          </n-button>
          <n-button v-if="canResolve" type="success" size="small" @click="handleResolve">
            {{ t('alert.resolve') }}
          </n-button>
          <n-button v-if="canClose" size="small" quaternary @click="handleClose">
            {{ t('alert.close') }}
          </n-button>
        </div>
      </div>

      <!-- Incident Lifecycle Bar -->
      <div class="lifecycle-bar">
        <template v-for="(step, idx) in lifecycleSteps" :key="step.key">
          <div class="lifecycle-step" :class="{ 'step--done': step.done, 'step--active': step.active }">
            <div class="step-dot" />
            <div class="step-label">{{ step.label }}</div>
            <div class="step-time">{{ step.time ? formatTime(step.time) : '—' }}</div>
          </div>
          <div v-if="idx < lifecycleSteps.length - 1" class="lifecycle-connector" :class="{ 'connector--done': step.done }" />
        </template>
      </div>
    </div>

    <!-- ═══ MAIN GRID ═══ -->
    <n-grid :x-gap="16" :y-gap="16" :cols="24" style="margin-top: 16px">

      <!-- ── Left Column ── -->
      <n-gi :span="16">

        <!-- Labels -->
        <div class="panel-card">
          <div class="panel-header">
            <span class="panel-title">{{ t('alert.labels') }}</span>
            <span class="panel-count">{{ Object.keys(event.labels || {}).length }}</span>
          </div>
          <div class="labels-grid">
            <div
              v-for="(value, key) in event.labels"
              :key="key"
              class="label-item"
              @click="copyLabel(String(key), String(value))"
              :title="`Click to copy: ${key}=${value}`"
            >
              <span class="label-key">{{ key }}</span>
              <span class="label-eq">=</span>
              <span class="label-val">{{ value }}</span>
              <n-icon :component="CopyOutline" size="10" class="label-copy-icon" />
            </div>
            <n-empty v-if="!event.labels || Object.keys(event.labels).length === 0" size="small" />
          </div>
        </div>

        <!-- Annotations -->
        <div
          v-if="event.annotations && Object.keys(event.annotations).length"
          class="panel-card"
          style="margin-top: 16px"
        >
          <div class="panel-header">
            <span class="panel-title">{{ t('alert.annotations') }}</span>
          </div>
          <div v-for="(value, key) in event.annotations" :key="key" class="annotation-block">
            <div class="annotation-key">{{ key }}</div>
            <div class="annotation-value">{{ value }}</div>
          </div>
        </div>

        <!-- Timeline -->
        <div class="panel-card" style="margin-top: 16px">
          <div class="panel-header">
            <span class="panel-title">{{ t('alert.timeline') }}</span>
            <span class="panel-count">{{ timeline.length }}</span>
          </div>

          <div class="timeline-list">
            <div v-for="item in timeline" :key="item.id" class="tl-item">
              <div
                class="tl-icon"
                :style="{ background: getTimelineColor(item.action) + '20', color: getTimelineColor(item.action), borderColor: getTimelineColor(item.action) + '40' }"
              >
                <n-icon :component="getTimelineIcon(item.action)" size="14" />
              </div>
              <div class="tl-connector" />
              <div class="tl-body">
                <div class="tl-header-row">
                  <span class="tl-action" :style="{ color: getTimelineColor(item.action) }">
                    {{ getTimelineLabel(item.action) }}
                  </span>
                  <span v-if="item.operator" class="tl-operator">
                    {{ item.operator.display_name || item.operator.username }}
                  </span>
                  <span class="tl-time">{{ formatTime(item.created_at) }}</span>
                </div>
                <div v-if="item.note" class="tl-note">{{ item.note }}</div>
              </div>
            </div>
            <n-empty v-if="!timeline.length" size="small" style="padding: 16px 0" />
          </div>

          <!-- Comment input -->
          <div class="comment-box">
            <n-input
              v-model:value="commentText"
              type="textarea"
              :placeholder="t('alert.addCommentPlaceholder')"
              :rows="2"
            />
            <div style="display:flex;justify-content:flex-end;margin-top:8px">
              <n-button type="primary" size="small" :disabled="!commentText.trim()" @click="handleComment">
                <template #icon><n-icon :component="ChatbubbleOutline" /></template>
                {{ t('alert.addComment') }}
              </n-button>
            </div>
          </div>
        </div>

      </n-gi>

      <!-- ── Right Column ── -->
      <n-gi :span="8">

        <!-- Responders -->
        <div class="panel-card">
          <div class="panel-header"><span class="panel-title">Responders</span></div>
          <div class="responders-list">
            <div class="responder-row" v-if="event.acked_by_user">
              <div class="responder-avatar">{{ (event.acked_by_user.display_name || '?').charAt(0).toUpperCase() }}</div>
              <div class="responder-info">
                <div class="responder-name">{{ event.acked_by_user.display_name || event.acked_by_user.username }}</div>
                <div class="responder-role">{{ t('alert.acknowledged') }}</div>
              </div>
              <div class="responder-time">{{ formatTime(event.acked_at) }}</div>
            </div>
            <div class="responder-row" v-if="event.assigned_to_user">
              <div class="responder-avatar" style="background: var(--sre-gradient-calm)">
                {{ (event.assigned_to_user.display_name || '?').charAt(0).toUpperCase() }}
              </div>
              <div class="responder-info">
                <div class="responder-name">{{ event.assigned_to_user.display_name || event.assigned_to_user.username }}</div>
                <div class="responder-role">{{ t('alert.assignedTo') }}</div>
              </div>
            </div>
            <div class="responder-row" v-if="event.oncall_user">
              <div class="responder-avatar" style="background: linear-gradient(135deg, var(--sre-aurora-3), #7c3aed)">
                {{ (event.oncall_user.display_name || '?').charAt(0).toUpperCase() }}
              </div>
              <div class="responder-info">
                <div class="responder-name">{{ event.oncall_user.display_name || event.oncall_user.username }}</div>
                <div class="responder-role">{{ t('alert.oncallUser') }}</div>
              </div>
            </div>
            <n-empty
              v-if="!event.acked_by_user && !event.assigned_to_user && !event.oncall_user"
              size="small"
              description="No responders yet"
              style="padding: 12px 0"
            />
          </div>
        </div>

        <!-- Event Details -->
        <div class="panel-card" style="margin-top: 16px">
          <div class="panel-header"><span class="panel-title">{{ t('alert.details') }}</span></div>
          <div class="details-list">
            <div class="details-row">
              <span class="details-label">ID</span>
              <span class="details-value">#{{ event.id }}</span>
            </div>
            <div class="details-row">
              <span class="details-label">{{ t('alert.source') }}</span>
              <span class="details-value">{{ event.source || '—' }}</span>
            </div>
            <div class="details-row">
              <span class="details-label">{{ t('alert.firedAt') }}</span>
              <span class="details-value">{{ formatTime(event.fired_at) }}</span>
            </div>
            <div class="details-row" v-if="event.acked_at">
              <span class="details-label">{{ t('alert.ackedAt') }}</span>
              <span class="details-value">{{ formatTime(event.acked_at) }}</span>
            </div>
            <div class="details-row" v-if="event.resolved_at">
              <span class="details-label">{{ t('alert.resolvedAt') }}</span>
              <span class="details-value">{{ formatTime(event.resolved_at) }}</span>
            </div>
            <div class="details-row" v-if="event.closed_at">
              <span class="details-label">{{ t('alert.closedAt') }}</span>
              <span class="details-value">{{ formatTime(event.closed_at) }}</span>
            </div>
            <div class="details-row">
              <span class="details-label">{{ t('alert.fireCount') }}</span>
              <span class="details-value fire-count-val">{{ event.fire_count }}×</span>
            </div>
            <div class="details-row" v-if="event.silenced_until">
              <span class="details-label">{{ t('alert.silence') }}</span>
              <span class="details-value" style="color: var(--sre-aurora-3)">{{ formatTime(event.silenced_until) }}</span>
            </div>
            <div class="details-row">
              <span class="details-label">{{ t('alert.fingerprint') }}</span>
              <code class="fp-code" @click="copyLabel('fingerprint', event.fingerprint)" title="Click to copy">
                {{ event.fingerprint.slice(0, 12) }}…
              </code>
            </div>
            <div class="details-row" v-if="event.generator_url">
              <span class="details-label">{{ t('alert.generatorUrl') }}</span>
              <a :href="event.generator_url" target="_blank" class="details-link">↗ Source</a>
            </div>
          </div>
        </div>

        <!-- AI Analysis -->
        <div class="panel-card" style="margin-top: 16px">
          <div class="panel-header">
            <span class="panel-title">{{ t('alert.aiAnalysis') }}</span>
            <n-button v-if="!aiReport && !aiReportLoading" size="tiny" type="primary" secondary @click="generateAIReport">
              {{ t('alert.generateReport') }}
            </n-button>
            <n-button v-if="aiReport" size="tiny" quaternary @click="generateAIReport">
              {{ t('alert.regenerateReport') }}
            </n-button>
          </div>
          <n-spin :show="aiReportLoading">
            <div v-if="!aiReport && !aiReportLoading && !aiReportError" class="ai-hint">
              {{ t('alert.aiAnalysisHint') }}
            </div>
            <n-alert v-if="aiReportError" type="error" :bordered="false" size="small">
              {{ aiReportError }}
            </n-alert>
            <div v-if="aiReport" class="ai-report">
              <p class="ai-summary">{{ aiReport.summary }}</p>
              <div v-if="aiReport.probable_causes?.length" class="ai-section">
                <div class="ai-section-title">{{ t('alert.aiProbableCauses') }}</div>
                <ul class="ai-list">
                  <li v-for="(c, i) in aiReport.probable_causes" :key="i">{{ c }}</li>
                </ul>
              </div>
              <div v-if="aiReport.impact" class="ai-section">
                <div class="ai-section-title">{{ t('alert.aiImpact') }}</div>
                <p class="ai-text">{{ aiReport.impact }}</p>
              </div>
              <div v-if="aiReport.recommended_steps?.length" class="ai-section">
                <div class="ai-section-title">{{ t('alert.aiRecommendedSteps') }}</div>
                <ol class="ai-list">
                  <li v-for="(s, i) in aiReport.recommended_steps" :key="i">{{ s }}</li>
                </ol>
              </div>
            </div>
          </n-spin>
        </div>

      </n-gi>
    </n-grid>

    <!-- Silence Modal -->
    <n-modal v-model:show="showSilenceModal" preset="card" :title="t('alert.silence')" style="width: 480px" :bordered="false">
      <n-form label-placement="top">
        <n-form-item :label="t('alert.silenceDuration')">
          <n-radio-group v-model:value="silenceDuration">
            <n-space>
              <n-radio-button v-for="opt in silenceDurationOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</n-radio-button>
            </n-space>
          </n-radio-group>
        </n-form-item>
        <n-form-item :label="t('alert.silenceReason')">
          <n-input v-model:value="silenceReason" type="textarea" :placeholder="t('alert.silenceReasonPlaceholder')" :rows="3" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space justify="end">
          <n-button @click="showSilenceModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="warning" :loading="silenceSaving" @click="handleSilence">{{ t('common.confirm') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- Assign Modal -->
    <n-modal v-model:show="showAssignModal" preset="card" :title="t('alert.assign')" style="width: 480px" :bordered="false">
      <n-form label-placement="top">
        <n-form-item :label="t('alert.assignTo')">
          <n-select v-model:value="assignUserId" :options="userOptions" :placeholder="t('alert.selectUser')" filterable />
        </n-form-item>
        <n-form-item :label="t('alert.assignNote')">
          <n-input v-model:value="assignNote" type="textarea" :placeholder="t('alert.assignNotePlaceholder')" :rows="3" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space justify="end">
          <n-button @click="showAssignModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="assignSaving" @click="handleAssign">{{ t('common.confirm') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>

  <div v-else style="padding: 60px; text-align: center">
    <n-spin v-if="loading" />
  </div>
</template>

<style scoped>
.event-detail { max-width: 1400px; }

/* ═══ INCIDENT BANNER ═══ */
.incident-banner {
  border-radius: 14px;
  padding: 16px 20px 0;
  margin-bottom: 0;
  position: relative;
  overflow: hidden;
}
.banner--critical {
  background: linear-gradient(135deg, var(--sre-critical-soft) 0%, rgba(239,68,68,0.06) 100%);
  border: 1px solid rgba(239,68,68,0.25);
}
.banner--warning {
  background: linear-gradient(135deg, var(--sre-warning-soft) 0%, rgba(245,158,11,0.06) 100%);
  border: 1px solid rgba(245,158,11,0.25);
}
.banner--info {
  background: linear-gradient(135deg, var(--sre-info-soft) 0%, rgba(59,130,246,0.04) 100%);
  border: 1px solid rgba(59,130,246,0.2);
}

.back-btn {
  font-size: 12px;
  color: var(--sre-text-secondary);
  margin-bottom: 12px;
}

.banner-body {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.banner-icon-wrap {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.banner--critical .banner-icon-wrap { background: var(--sre-critical-soft); color: var(--sre-critical); }
.banner--warning  .banner-icon-wrap { background: var(--sre-warning-soft); color: var(--sre-warning); }
.banner--info     .banner-icon-wrap { background: var(--sre-info-soft); color: var(--sre-info); }

.banner-text { flex: 1; min-width: 0; }

.banner-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--sre-text-primary);
  line-height: 1.2;
  margin-bottom: 10px;
  word-break: break-word;
}

.banner-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--sre-text-secondary);
  background: rgba(128,128,128,0.1);
  padding: 3px 8px;
  border-radius: 6px;
  font-weight: 500;
}
.fire-count { color: var(--sre-critical); background: var(--sre-critical-soft); }

.banner-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex-shrink: 0;
  align-items: flex-end;
}

/* Lifecycle bar */
.lifecycle-bar {
  display: flex;
  align-items: center;
  padding: 14px 4px 16px;
  border-top: 1px solid rgba(128,128,128,0.12);
  margin-top: 4px;
  gap: 0;
}

.lifecycle-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  flex: 1;
  opacity: 0.4;
  transition: opacity 0.2s;
}
.lifecycle-step.step--done { opacity: 0.8; }
.lifecycle-step.step--active { opacity: 1; }

.step-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--sre-text-secondary);
  border: 2px solid rgba(128,128,128,0.3);
  transition: all 0.2s;
}
.step--done .step-dot { background: var(--sre-success); border-color: rgba(16,185,129,0.3); }
.step--active .step-dot {
  background: var(--sre-success);
  box-shadow: 0 0 0 4px rgba(16,185,129,0.2);
  animation: step-pulse 2s ease-in-out infinite;
}
@keyframes step-pulse {
  0%, 100% { box-shadow: 0 0 0 3px rgba(16,185,129,0.2); }
  50% { box-shadow: 0 0 0 6px rgba(16,185,129,0.1); }
}
.step-label { font-size: 11px; color: var(--sre-text-secondary); font-weight: 500; }
.step-time  { font-size: 10px; color: var(--sre-text-secondary); opacity: 0.7; text-align: center; }

.lifecycle-connector {
  flex: 2;
  height: 2px;
  background: rgba(128,128,128,0.2);
  margin-bottom: 28px;
  transition: background 0.2s;
}
.lifecycle-connector.connector--done { background: rgba(16,185,129,0.4); }

/* ═══ PANEL CARDS ═══ */
.panel-card {
  background: var(--sre-bg-card);
  border-radius: 12px;
  padding: 16px 18px;
}
.panel-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}
.panel-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--sre-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.6px;
  flex: 1;
}
.panel-count {
  font-size: 11px;
  background: rgba(128,128,128,0.1);
  color: var(--sre-text-secondary);
  padding: 1px 7px;
  border-radius: 10px;
}

/* Labels */
.labels-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.label-item {
  display: inline-flex;
  align-items: center;
  font-size: 12px;
  border-radius: 6px;
  overflow: hidden;
  cursor: pointer;
  border: 1px solid rgba(128,128,128,0.12);
  transition: border-color 0.15s, box-shadow 0.15s;
  max-width: 320px;
}
.label-item:hover {
  border-color: var(--sre-primary-ring);
  box-shadow: 0 0 0 2px var(--sre-primary-soft);
}
.label-key {
  background: rgba(128,128,128,0.1);
  padding: 4px 7px;
  color: var(--sre-text-secondary);
  white-space: nowrap;
}
.label-eq {
  background: rgba(128,128,128,0.07);
  padding: 4px 2px;
  color: var(--sre-text-secondary);
  font-size: 10px;
}
.label-val {
  background: var(--sre-primary-soft);
  padding: 4px 7px;
  color: var(--sre-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 180px;
}
.label-copy-icon {
  padding: 4px 5px 4px 3px;
  background: var(--sre-primary-soft);
  color: var(--sre-text-secondary);
  opacity: 0;
  transition: opacity 0.15s;
}
.label-item:hover .label-copy-icon { opacity: 1; }

/* Annotations */
.annotation-block {
  margin-bottom: 12px;
  padding: 10px 12px;
  background: rgba(128,128,128,0.05);
  border-radius: 8px;
  border-left: 3px solid var(--sre-info);
}
.annotation-key {
  font-size: 11px;
  color: var(--sre-text-secondary);
  font-weight: 600;
  margin-bottom: 5px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
}
.annotation-value {
  font-size: 13px;
  color: var(--sre-text-primary);
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.5;
}

/* Timeline */
.timeline-list { display: flex; flex-direction: column; gap: 0; }

.tl-item {
  display: flex;
  align-items: stretch;
  gap: 0;
  position: relative;
}

.tl-icon {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 1.5px solid;
  margin-top: 4px;
  z-index: 1;
}

.tl-connector {
  width: 1px;
  background: rgba(128,128,128,0.15);
  margin: 0 13px;
  flex-shrink: 0;
}
.tl-item:last-child .tl-connector { display: none; }

.tl-body {
  flex: 1;
  padding: 4px 0 16px 2px;
}
.tl-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.tl-action {
  font-size: 13px;
  font-weight: 600;
}
.tl-operator {
  font-size: 12px;
  color: var(--sre-text-secondary);
  background: rgba(128,128,128,0.08);
  padding: 1px 7px;
  border-radius: 5px;
}
.tl-time {
  font-size: 11px;
  color: var(--sre-text-secondary);
  margin-left: auto;
}
.tl-note {
  font-size: 12px;
  color: var(--sre-text-secondary);
  margin-top: 4px;
  padding: 6px 10px;
  background: rgba(128,128,128,0.06);
  border-radius: 6px;
  white-space: pre-wrap;
}

/* Comment box */
.comment-box {
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid var(--sre-border);
}

/* Responders */
.responders-list { display: flex; flex-direction: column; gap: 10px; }
.responder-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  background: rgba(128,128,128,0.05);
  border-radius: 8px;
}
.responder-avatar {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: var(--sre-gradient-brand);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.responder-info { flex: 1; }
.responder-name { font-size: 13px; font-weight: 600; color: var(--sre-text-primary); }
.responder-role { font-size: 11px; color: var(--sre-text-secondary); margin-top: 1px; }
.responder-time { font-size: 11px; color: var(--sre-text-secondary); }

/* Details list */
.details-list { display: flex; flex-direction: column; gap: 8px; }
.details-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 12px;
}
.details-label {
  color: var(--sre-text-secondary);
  white-space: nowrap;
  flex-shrink: 0;
  width: 82px;
}
.details-value {
  color: var(--sre-text-primary);
  flex: 1;
  word-break: break-all;
}
.fire-count-val { color: var(--sre-critical); font-weight: 700; font-size: 14px; }
.fp-code {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 11px;
  color: var(--sre-text-secondary);
  background: rgba(128,128,128,0.1);
  padding: 2px 6px;
  border-radius: 4px;
  cursor: pointer;
  flex: 1;
}
.fp-code:hover { color: var(--sre-primary); background: var(--sre-primary-soft); }
.details-link { color: var(--sre-info); font-size: 12px; text-decoration: none; }
.details-link:hover { text-decoration: underline; }

/* AI report */
.ai-hint {
  font-size: 12px;
  color: var(--sre-text-secondary);
  text-align: center;
  padding: 12px 0;
  line-height: 1.6;
}
.ai-report { font-size: 13px; }
.ai-summary {
  color: var(--sre-text-primary);
  line-height: 1.6;
  margin: 0 0 12px 0;
  padding: 10px 12px;
  background: rgba(128,128,128,0.05);
  border-radius: 8px;
  white-space: pre-wrap;
}
.ai-section { margin-bottom: 12px; }
.ai-section-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--sre-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 6px;
}
.ai-list {
  margin: 0;
  padding-left: 18px;
  color: var(--sre-text-primary);
  line-height: 1.7;
}
.ai-text {
  color: var(--sre-text-primary);
  line-height: 1.6;
  margin: 0;
}
</style>
