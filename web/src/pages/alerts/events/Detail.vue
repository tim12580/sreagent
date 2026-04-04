<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { alertEventApi, userApi, aiApi } from '@/api'
import type { AlertEvent, AlertTimeline, User } from '@/types'
import { formatTime, formatDuration } from '@/utils/format'
import { getSeverityType, getSeverityColor, getStatusColor, statusTagColor, getStatusLabelKey, getTimelineType } from '@/utils/alert'

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
  { label: '30m', value: 30 },
  { label: '1h', value: 60 },
  { label: '2h', value: 120 },
  { label: '6h', value: 360 },
  { label: '12h', value: 720 },
  { label: '24h', value: 1440 },
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

const canAck = computed(() => event.value?.status === 'firing')
const canAssign = computed(() => event.value?.status === 'firing' || event.value?.status === 'acknowledged')
const canSilence = computed(() => event.value?.status === 'firing' || event.value?.status === 'acknowledged')
const canResolve = computed(() => event.value?.status !== 'resolved' && event.value?.status !== 'closed')
const canClose = computed(() => event.value?.status !== 'closed')

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
  } catch (_err) {
    // silently fail
  }
}

async function handleAck() {
  try {
    await alertEventApi.acknowledge(eventId)
    message.success(t('alert.alertAcknowledged'))
    fetchEvent()
    fetchTimeline()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleResolve() {
  try {
    await alertEventApi.resolve(eventId, { resolution: t('alert.manuallyResolved') })
    message.success(t('alert.alertResolved'))
    fetchEvent()
    fetchTimeline()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleClose() {
  try {
    await alertEventApi.close(eventId)
    message.success(t('alert.alertClosed'))
    fetchEvent()
    fetchTimeline()
  } catch (err: any) {
    message.error(err.message)
  }
}

function openSilenceModal() {
  silenceDuration.value = 60
  silenceReason.value = ''
  showSilenceModal.value = true
}

async function handleSilence() {
  if (!silenceReason.value.trim()) {
    message.warning(t('alert.silenceReasonPlaceholder'))
    return
  }
  silenceSaving.value = true
  try {
    await alertEventApi.silence(eventId, {
      duration_minutes: silenceDuration.value,
      reason: silenceReason.value,
    })
    message.success(t('alert.silenceSuccess'))
    showSilenceModal.value = false
    fetchEvent()
    fetchTimeline()
  } catch (err: any) {
    message.error(err.message)
  } finally {
    silenceSaving.value = false
  }
}

function openAssignModal() {
  assignUserId.value = null
  assignNote.value = ''
  showAssignModal.value = true
  if (users.value.length === 0) fetchUsers()
}

async function handleAssign() {
  if (!assignUserId.value) {
    message.warning(t('alert.selectUser'))
    return
  }
  assignSaving.value = true
  try {
    await alertEventApi.assign(eventId, {
      assign_to: assignUserId.value,
      note: assignNote.value || undefined,
    })
    message.success(t('alert.assignSuccess'))
    showAssignModal.value = false
    fetchEvent()
    fetchTimeline()
  } catch (err: any) {
    message.error(err.message)
  } finally {
    assignSaving.value = false
  }
}

async function handleComment() {
  if (!commentText.value.trim()) return
  try {
    await alertEventApi.comment(eventId, { note: commentText.value })
    commentText.value = ''
    message.success(t('alert.commentAdded'))
    fetchTimeline()
  } catch (err: any) {
    message.error(err.message)
  }
}

onMounted(() => {
  fetchEvent()
  fetchTimeline()
})

// ===== AI Analysis =====
const aiReport = ref<{ summary: string; probable_causes: string[]; impact: string; recommended_steps: string[] } | null>(null)
const aiReportLoading = ref(false)
const aiReportError = ref('')

async function generateAIReport() {
  aiReportLoading.value = true
  aiReportError.value = ''
  aiReport.value = null
  try {
    const res = await aiApi.generateReport(eventId)
    aiReport.value = res.data.data ?? null
  } catch (err: any) {
    aiReportError.value = err.message || t('alert.aiReportError')
  } finally {
    aiReportLoading.value = false
  }
}
</script>

<template>
  <div class="event-detail" v-if="event">
    <!-- Back button -->
    <n-button text @click="router.back()" style="margin-bottom: 12px">
      &larr; {{ t('alert.backToEvents') }}
    </n-button>

    <!-- Alert Header -->
    <div class="alert-header">
      <div class="header-left">
        <h2 class="alert-title">{{ event.alert_name }}</h2>
        <div class="meta-row">
          <n-tag :type="getSeverityType(event.severity)" size="small" round>
            {{ event.severity.toUpperCase() }}
          </n-tag>
          <n-tag
            size="small"
            :bordered="false"
            :color="statusTagColor(event.status)"
          >
            {{ t(getStatusLabelKey(event.status)) }}
          </n-tag>
          <n-text depth="3" style="font-size: 13px">{{ eventDuration }}</n-text>
        </div>
      </div>
      <!-- Action Buttons -->
      <div class="header-actions">
        <n-button v-if="canAck" type="primary" size="small" @click="handleAck">{{ t('alert.acknowledge') }}</n-button>
        <n-button v-if="canAssign" type="info" size="small" @click="openAssignModal">{{ t('alert.assign') }}</n-button>
        <n-button v-if="canSilence" type="warning" size="small" secondary @click="openSilenceModal">{{ t('alert.silence') }}</n-button>
        <n-button v-if="canResolve" type="success" size="small" @click="handleResolve">{{ t('alert.resolve') }}</n-button>
        <n-button v-if="canClose" size="small" @click="handleClose">{{ t('alert.close') }}</n-button>
      </div>
    </div>

    <n-grid :x-gap="16" :y-gap="16" :cols="24">
      <!-- Left Column (2/3) -->
      <n-gi :span="16">
        <!-- Labels -->
        <n-card :title="t('alert.labels')" :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px">
          <div class="labels-grid">
            <div v-for="(value, key) in event.labels" :key="key" class="label-item">
              <span class="label-key">{{ key }}</span>
              <span class="label-value">{{ value }}</span>
            </div>
            <n-empty v-if="!event.labels || Object.keys(event.labels).length === 0" size="small" />
          </div>
        </n-card>

        <!-- Annotations -->
        <n-card
          v-if="event.annotations && Object.keys(event.annotations).length"
          :title="t('alert.annotations')"
          :bordered="false"
          style="background: var(--sre-bg-card); border-radius: 12px; margin-top: 16px"
        >
          <div v-for="(value, key) in event.annotations" :key="key" class="annotation-block">
            <div class="annotation-key">{{ key }}</div>
            <div class="annotation-value">{{ value }}</div>
          </div>
        </n-card>

        <!-- Timeline -->
        <n-card :title="t('alert.timeline')" :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px; margin-top: 16px">
          <n-timeline v-if="timeline.length > 0">
            <n-timeline-item
              v-for="item in timeline"
              :key="item.id"
              :type="getTimelineType(item.action)"
              :time="formatTime(item.created_at)"
            >
              <template #header>
                <span style="font-weight: 500">{{ item.action.charAt(0).toUpperCase() + item.action.slice(1) }}</span>
                <span v-if="item.operator" style="color: var(--sre-text-secondary); margin-left: 8px; font-size: 12px">
                  {{ item.operator.display_name }}
                </span>
              </template>
              <template #default>
                <span v-if="item.note" style="color: var(--sre-text-secondary)">{{ item.note }}</span>
              </template>
            </n-timeline-item>
          </n-timeline>
          <n-empty v-else size="small" />

          <!-- Comment input -->
          <div class="comment-box">
            <n-input
              v-model:value="commentText"
              type="textarea"
              :placeholder="t('alert.addCommentPlaceholder')"
              :rows="2"
            />
            <n-button type="primary" size="small" @click="handleComment" style="margin-top: 8px">
              {{ t('alert.addComment') }}
            </n-button>
          </div>
        </n-card>
      </n-gi>

      <!-- Right Column (1/3) -->
      <n-gi :span="8">
        <!-- Details Card -->
        <n-card :title="t('alert.details')" :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px">
          <n-descriptions :column="1" label-placement="left" size="small" :label-style="{ color: 'var(--sre-text-secondary)', width: '90px' }">
            <n-descriptions-item label="ID">{{ event.id }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.fingerprint')">
              <n-text code style="font-size: 11px; word-break: break-all">{{ event.fingerprint }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item :label="t('alert.source')">{{ event.source || '-' }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.generatorUrl')">
              <a v-if="event.generator_url" :href="event.generator_url" target="_blank" style="color: var(--sre-info); font-size: 12px; word-break: break-all">
                {{ event.generator_url }}
              </a>
              <span v-else>-</span>
            </n-descriptions-item>
            <n-descriptions-item :label="t('alert.firedAt')">{{ formatTime(event.fired_at) }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.ackedBy')">{{ event.acked_by_user?.display_name || '-' }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.ackedAt')">{{ formatTime(event.acked_at) }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.assignedTo')">{{ event.assigned_to_user?.display_name || '-' }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.resolvedAt')">{{ formatTime(event.resolved_at) }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.closedAt')">{{ formatTime(event.closed_at) }}</n-descriptions-item>
            <n-descriptions-item :label="t('alert.fireCount')">{{ event.fire_count }}</n-descriptions-item>
          </n-descriptions>
        </n-card>

        <!-- AI Analysis Card -->
        <n-card :title="t('alert.aiAnalysis')" :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px; margin-top: 16px">
          <n-spin :show="aiReportLoading">
            <div v-if="!aiReport && !aiReportLoading && !aiReportError" style="text-align: center; padding: 20px 0">
              <n-text depth="3" style="display: block; margin-bottom: 12px">{{ t('alert.aiAnalysisHint') }}</n-text>
              <n-button type="primary" secondary size="small" @click="generateAIReport">{{ t('alert.generateReport') }}</n-button>
            </div>
            <n-alert v-if="aiReportError" type="error" :bordered="false" style="margin-bottom: 12px">
              {{ aiReportError }}
              <n-button size="tiny" style="margin-left: 8px" @click="generateAIReport">{{ t('common.retry') }}</n-button>
            </n-alert>
            <div v-if="aiReport">
              <n-thing>
                <template #description>
                  <n-text depth="2" style="font-size: 13px; white-space: pre-wrap">{{ aiReport.summary }}</n-text>
                </template>
              </n-thing>
              <n-divider />
              <div v-if="aiReport.probable_causes?.length">
                <n-text strong>{{ t('alert.aiProbableCauses') }}</n-text>
                <ul style="margin: 8px 0 0 16px; padding: 0">
                  <li v-for="(cause, i) in aiReport.probable_causes" :key="i" style="font-size: 13px; margin-bottom: 4px">{{ cause }}</li>
                </ul>
              </div>
              <div v-if="aiReport.impact" style="margin-top: 12px">
                <n-text strong>{{ t('alert.aiImpact') }}</n-text>
                <n-text depth="2" style="font-size: 13px; display: block; margin-top: 4px">{{ aiReport.impact }}</n-text>
              </div>
              <div v-if="aiReport.recommended_steps?.length" style="margin-top: 12px">
                <n-text strong>{{ t('alert.aiRecommendedSteps') }}</n-text>
                <ol style="margin: 8px 0 0 16px; padding: 0">
                  <li v-for="(step, i) in aiReport.recommended_steps" :key="i" style="font-size: 13px; margin-bottom: 4px">{{ step }}</li>
                </ol>
              </div>
              <div style="margin-top: 16px; text-align: right">
                <n-button size="tiny" @click="generateAIReport">{{ t('alert.regenerateReport') }}</n-button>
              </div>
            </div>
          </n-spin>
        </n-card>
      </n-gi>
    </n-grid>

    <!-- Silence Modal -->
    <n-modal v-model:show="showSilenceModal" preset="card" :title="t('alert.silence')" style="width: 480px" :bordered="false">
      <n-form label-placement="top">
        <n-form-item :label="t('alert.silenceDuration')">
          <n-radio-group v-model:value="silenceDuration">
            <n-space>
              <n-radio-button v-for="opt in silenceDurationOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </n-radio-button>
            </n-space>
          </n-radio-group>
        </n-form-item>
        <n-form-item :label="t('alert.silenceReason')">
          <n-input
            v-model:value="silenceReason"
            type="textarea"
            :placeholder="t('alert.silenceReasonPlaceholder')"
            :rows="3"
          />
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
          <n-select
            v-model:value="assignUserId"
            :options="userOptions"
            :placeholder="t('alert.selectUser')"
            filterable
          />
        </n-form-item>
        <n-form-item :label="t('alert.assignNote')">
          <n-input
            v-model:value="assignNote"
            type="textarea"
            :placeholder="t('alert.assignNotePlaceholder')"
            :rows="3"
          />
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

  <!-- Loading state -->
  <div v-else style="padding: 60px; text-align: center">
    <n-spin v-if="loading" />
  </div>
</template>

<style scoped>
.event-detail {
  max-width: 1400px;
}

.alert-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
  padding: 16px 20px;
  background: var(--sre-bg-card);
  border-radius: 12px;
}

.alert-title {
  font-size: 22px;
  font-weight: 600;
  margin: 0 0 8px 0;
  color: var(--sre-text-primary);
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.labels-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.label-item {
  display: flex;
  font-size: 12px;
  border-radius: 6px;
  overflow: hidden;
}

.label-key {
  background: rgba(128, 128, 128, 0.1);
  padding: 4px 8px;
  color: var(--sre-text-secondary);
}

.label-value {
  background: rgba(24, 160, 88, 0.15);
  padding: 4px 8px;
  color: var(--sre-text-primary);
}

.annotation-block {
  margin-bottom: 14px;
  padding: 10px 12px;
  background: rgba(128, 128, 128, 0.06);
  border-radius: 8px;
}

.annotation-key {
  font-size: 12px;
  color: var(--sre-text-secondary);
  margin-bottom: 4px;
  font-weight: 500;
}

.annotation-value {
  font-size: 13px;
  color: var(--sre-text-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.comment-box {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--sre-border);
}
</style>
