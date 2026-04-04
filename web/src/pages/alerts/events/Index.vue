<script setup lang="ts">
import { h, ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NTag, NButton, NSpace } from 'naive-ui'

type RowKey = string | number
import { useI18n } from 'vue-i18n'
import { alertEventApi } from '@/api'
import type { AlertEvent, AlertViewMode } from '@/types'
import { formatTime, formatDuration } from '@/utils/format'
import { getSeverityType, getStatusLabelKey, statusTagColor, severityRowClass } from '@/utils/alert'
import PageHeader from '@/components/common/PageHeader.vue'
import { RefreshOutline } from '@vicons/ionicons5'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const message = useMessage()
const { t } = useI18n()
const authStore = useAuthStore()

// View mode
const viewMode = ref<AlertViewMode>('mine')

const canViewAll = computed(() =>
  authStore.user?.role === 'admin' || authStore.user?.role === 'global_viewer'
)

const viewModeOptions = computed(() => {
  const opts = [
    { label: t('alert.myAlerts'), value: 'mine' as AlertViewMode },
    { label: t('alert.unassigned'), value: 'unassigned' as AlertViewMode },
  ]
  if (canViewAll.value) {
    opts.push({ label: t('alert.allAlerts'), value: 'all' as AlertViewMode })
  }
  return opts
})

function handleViewModeChange(mode: AlertViewMode) {
  viewMode.value = mode
  page.value = 1
  fetchEvents()
}
const loading = ref(false)
const events = ref<AlertEvent[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const checkedRowKeys = ref<number[]>([])

// Filters
const statusFilter = ref<string[]>([])
const severityFilter = ref<string[]>([])
const alertNameSearch = ref('')
const sourceFilter = ref('')
const timeRangePreset = ref('24h')
const customRange = ref<[number, number] | null>(null)

const statusOptions = [
  { label: () => t('alert.firing'), value: 'firing' },
  { label: () => t('alert.acknowledged'), value: 'acknowledged' },
  { label: () => t('alert.assigned'), value: 'assigned' },
  { label: () => t('alert.resolved'), value: 'resolved' },
  { label: () => t('alert.closed'), value: 'closed' },
  { label: () => t('alert.silenced'), value: 'silenced' },
]

const severityOptions = [
  { label: () => t('alert.critical'), value: 'critical' },
  { label: () => t('alert.warning'), value: 'warning' },
  { label: () => t('alert.info'), value: 'info' },
]

const timePresets = [
  { label: () => t('alert.last1h'), value: '1h' },
  { label: () => t('alert.last6h'), value: '6h' },
  { label: () => t('alert.last24h'), value: '24h' },
  { label: () => t('alert.last7d'), value: '7d' },
  { label: () => t('alert.last30d'), value: '30d' },
]

function getTimeRange(): { start_time?: string; end_time?: string } {
  if (timeRangePreset.value === 'custom' && customRange.value) {
    return {
      start_time: new Date(customRange.value[0]).toISOString(),
      end_time: new Date(customRange.value[1]).toISOString(),
    }
  }
  const now = new Date()
  const map: Record<string, number> = {
    '1h': 3600000,
    '6h': 21600000,
    '24h': 86400000,
    '7d': 604800000,
    '30d': 2592000000,
  }
  const ms = map[timeRangePreset.value]
  if (ms) {
    return { start_time: new Date(now.getTime() - ms).toISOString() }
  }
  return {}
}

function calcDuration(row: AlertEvent): string {
  const firedAt = new Date(row.fired_at).getTime()
  if (row.status === 'resolved' || row.status === 'closed') {
    const end = row.resolved_at ? new Date(row.resolved_at).getTime() : (row.closed_at ? new Date(row.closed_at).getTime() : Date.now())
    return formatDuration(Math.floor((end - firedAt) / 1000))
  }
  return formatDuration(Math.floor((Date.now() - firedAt) / 1000))
}

const columns = [
  { type: 'selection' as const, width: 40 },
  {
    title: () => t('alert.severity'),
    key: 'severity',
    width: 90,
    render: (row: AlertEvent) =>
      h(NTag, { type: getSeverityType(row.severity), size: 'small', round: true }, { default: () => row.severity.toUpperCase() }),
  },
  {
    title: () => t('alert.alertName'),
    key: 'alert_name',
    ellipsis: { tooltip: true },
    minWidth: 180,
    render: (row: AlertEvent) => h('a', {
      style: 'color: var(--sre-info); cursor: pointer; text-decoration: none; font-weight: 500',
      onClick: () => router.push(`/alerts/events/${row.id}`),
    }, row.alert_name),
  },
  {
    title: () => t('common.status'),
    key: 'status',
    width: 100,
    render: (row: AlertEvent) =>
      h(NTag, {
        size: 'small',
        bordered: false,
        color: statusTagColor(row.status),
      }, { default: () => t(getStatusLabelKey(row.status)) }),
  },
  {
    title: () => t('alert.source'),
    key: 'source',
    width: 120,
    ellipsis: { tooltip: true },
  },
  {
    title: () => t('alert.firedAt'),
    key: 'fired_at',
    width: 170,
    render: (row: AlertEvent) => h('span', { style: 'font-size: 12px' }, formatTime(row.fired_at)),
  },
  {
    title: () => t('alert.duration'),
    key: 'duration',
    width: 100,
    render: (row: AlertEvent) => h('span', { style: 'font-size: 12px; color: var(--sre-text-secondary)' }, calcDuration(row)),
  },
  {
    title: () => t('alert.fireCount'),
    key: 'fire_count',
    width: 60,
    align: 'center' as const,
  },
  {
    title: () => t('alert.ackedBy'),
    key: 'acked_by',
    width: 100,
    render: (row: AlertEvent) =>
      h('span', { style: 'font-size: 12px' }, row.acked_by_user?.display_name || '-'),
  },
  {
    title: () => t('alert.oncallUser'),
    key: 'oncall_user',
    width: 110,
    render: (row: AlertEvent) => {
      if (row.is_dispatched && row.oncall_user) {
        return h('span', { style: 'font-size: 12px' }, row.oncall_user.display_name || row.oncall_user.username)
      }
      return h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => t('alert.notDispatched') })
    },
  },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 180,
    render: (row: AlertEvent) => {
      const buttons: any[] = []
      if (row.status === 'firing') {
        buttons.push(
          h(NButton, { size: 'tiny', type: 'primary', secondary: true, onClick: () => handleAck(row.id) }, { default: () => t('alert.ack') })
        )
      }
      if (row.status === 'firing' || row.status === 'acknowledged') {
        buttons.push(
          h(NButton, { size: 'tiny', type: 'success', secondary: true, onClick: () => handleResolve(row.id) }, { default: () => t('alert.resolve') })
        )
      }
      if (row.status !== 'closed' && row.status !== 'resolved') {
        buttons.push(
          h(NButton, { size: 'tiny', secondary: true, onClick: () => handleClose(row.id) }, { default: () => t('alert.close') })
        )
      }
      buttons.push(
        h(NButton, { size: 'tiny', quaternary: true, onClick: () => router.push(`/alerts/events/${row.id}`) }, { default: () => t('alert.detail') })
      )
      return h(NSpace, { size: 4 }, { default: () => buttons })
    },
  },
]

async function fetchEvents() {
  loading.value = true
  try {
    const timeRange = getTimeRange()
    const { data } = await alertEventApi.list({
      page: page.value,
      page_size: pageSize.value,
      status: statusFilter.value.length ? statusFilter.value : undefined,
      severity: severityFilter.value.length ? severityFilter.value : undefined,
      alert_name: alertNameSearch.value || undefined,
      source: sourceFilter.value || undefined,
      view_mode: viewMode.value,
      ...timeRange,
    })
    events.value = data.data.list || []
    total.value = data.data.total
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

async function handleAck(id: number) {
  try {
    await alertEventApi.acknowledge(id)
    message.success(t('alert.alertAcknowledged'))
    fetchEvents()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleResolve(id: number) {
  try {
    await alertEventApi.resolve(id, { resolution: t('alert.manuallyResolved') })
    message.success(t('alert.alertResolved'))
    fetchEvents()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleClose(id: number) {
  try {
    await alertEventApi.close(id)
    message.success(t('alert.alertClosed'))
    fetchEvents()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleBatchAck() {
  if (checkedRowKeys.value.length === 0) return
  try {
    await alertEventApi.batchAcknowledge(checkedRowKeys.value)
    message.success(t('alert.batchAckSuccess'))
    checkedRowKeys.value = []
    fetchEvents()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleBatchClose() {
  if (checkedRowKeys.value.length === 0) return
  try {
    await alertEventApi.batchClose(checkedRowKeys.value)
    message.success(t('alert.batchCloseSuccess'))
    checkedRowKeys.value = []
    fetchEvents()
  } catch (err: any) {
    message.error(err.message)
  }
}

function resetFilters() {
  statusFilter.value = []
  severityFilter.value = []
  alertNameSearch.value = ''
  sourceFilter.value = ''
  timeRangePreset.value = '24h'
  customRange.value = null
  page.value = 1
  fetchEvents()
}

function handleTimePreset(preset: string) {
  timeRangePreset.value = preset
  if (preset !== 'custom') {
    customRange.value = null
  }
  page.value = 1
  fetchEvents()
}

function handleCustomRange(val: [number, number] | null) {
  customRange.value = val
  if (val) {
    timeRangePreset.value = 'custom'
    page.value = 1
    fetchEvents()
  }
}

// Auto-refresh every 30 seconds
let refreshTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  fetchEvents()
  refreshTimer = setInterval(fetchEvents, 30000)
})
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})

const selectedText = computed(() => t('alert.selectedCount', { n: checkedRowKeys.value.length }))

function onCheckedRowKeysUpdate(keys: RowKey[]) {
  checkedRowKeys.value = keys as number[]
}
</script>

<template>
  <div class="events-page">
    <PageHeader :title="t('alert.events')" :subtitle="t('alert.eventsSubtitle')">
      <template #actions>
        <n-text depth="3">{{ t('alert.totalAlerts', { n: total }) }}</n-text>
        <n-button @click="fetchEvents" :loading="loading">
          <template #icon><n-icon :component="RefreshOutline" /></template>
          {{ t('common.refresh') }}
        </n-button>
      </template>
    </PageHeader>

    <!-- View mode tabs -->
    <div class="view-mode-bar">
      <n-radio-group :value="viewMode" @update:value="handleViewModeChange" size="medium">
        <n-radio-button
          v-for="opt in viewModeOptions"
          :key="opt.value"
          :value="opt.value"
          :label="opt.label"
        />
      </n-radio-group>
    </div>

    <!-- Filter bar -->
    <div class="filter-bar">
      <n-select
        v-model:value="statusFilter"
        :options="statusOptions"
        multiple
        :placeholder="t('common.status')"
        clearable
        style="width: 220px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-select
        v-model:value="severityFilter"
        :options="severityOptions"
        multiple
        :placeholder="t('alert.severity')"
        clearable
        style="width: 200px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-input
        v-model:value="alertNameSearch"
        :placeholder="t('alert.alertNameSearch')"
        clearable
        style="width: 200px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-input
        v-model:value="sourceFilter"
        :placeholder="t('alert.sourceFilter')"
        clearable
        style="width: 160px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-button quaternary @click="resetFilters">{{ t('alert.resetFilters') }}</n-button>
    </div>

    <!-- Time range quick buttons -->
    <div class="time-range-bar">
      <n-space size="small">
        <n-button
          v-for="preset in timePresets"
          :key="preset.value"
          size="small"
          :type="timeRangePreset === preset.value ? 'primary' : 'default'"
          :secondary="timeRangePreset === preset.value"
          @click="handleTimePreset(preset.value)"
        >
          {{ preset.label() }}
        </n-button>
        <n-date-picker
          type="datetimerange"
          :value="customRange"
          clearable
          size="small"
          style="width: 340px"
          @update:value="handleCustomRange"
        />
      </n-space>
    </div>

    <!-- Batch actions bar -->
    <div v-if="checkedRowKeys.length > 0" class="batch-bar">
      <n-space align="center" size="small">
        <n-text depth="3">{{ selectedText }}</n-text>
        <n-button size="small" type="primary" @click="handleBatchAck">{{ t('alert.batchAck') }}</n-button>
        <n-button size="small" type="error" @click="handleBatchClose">{{ t('alert.batchClose') }}</n-button>
      </n-space>
    </div>

    <!-- Events Table -->
    <n-card :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px">
      <n-data-table
        :loading="loading"
        :columns="columns"
        :data="events"
        :row-key="(row: AlertEvent) => row.id"
        :row-class-name="severityRowClass"
        :checked-row-keys="checkedRowKeys"
        @update:checked-row-keys="onCheckedRowKeysUpdate"
        :bordered="false"
        :pagination="{
          page: page,
          pageSize: pageSize,
          itemCount: total,
          showSizePicker: true,
          pageSizes: [20, 50, 100],
          onChange: (p: number) => { page = p; fetchEvents() },
          onUpdatePageSize: (s: number) => { pageSize = s; page = 1; fetchEvents() },
        }"
      />
    </n-card>
  </div>
</template>

<style scoped>
.events-page {
  max-width: 1400px;
}

.view-mode-bar {
  margin-bottom: 14px;
}

.filter-bar {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
  align-items: center;
}

.time-range-bar {
  margin-bottom: 12px;
}

.batch-bar {
  background: rgba(99, 125, 255, 0.1);
  border: 1px solid rgba(99, 125, 255, 0.25);
  border-radius: 8px;
  padding: 8px 16px;
  margin-bottom: 12px;
}

:deep(.row-critical) {
  background-color: rgba(232, 128, 128, 0.04) !important;
}

:deep(.row-warning) {
  background-color: rgba(242, 201, 125, 0.04) !important;
}
</style>
