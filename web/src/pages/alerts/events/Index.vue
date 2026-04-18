<script setup lang="ts">
import { h, ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NTag, NButton, NSpace } from 'naive-ui'

type RowKey = string | number
import { useI18n } from 'vue-i18n'
import { alertEventApi, alertExportApi, alertGroupsApi } from '@/api'
import type { AlertEvent, AlertViewMode, AlertGroupItem } from '@/types'
import { formatTime, formatDuration } from '@/utils/format'
import { getSeverityType, getStatusLabelKey, statusTagColor, severityRowClass } from '@/utils/alert'
import PageHeader from '@/components/common/PageHeader.vue'
import { RefreshOutline, OptionsOutline, AlertCircleOutline, DownloadOutline, LayersOutline, ListOutline } from '@vicons/ionicons5'
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
const showFilters = ref(true)
const statusFilter = ref<string[]>([])
const severityFilter = ref<string[]>([])
const alertNameSearch = ref('')
const sourceFilter = ref('')
const timeRangePreset = ref('24h')
const customRange = ref<[number, number] | null>(null)

// Count by status from current result (rough breakdown for badge)
const statusCounts = computed(() => {
  const m: Record<string, number> = { firing: 0, acknowledged: 0, assigned: 0, resolved: 0, closed: 0, silenced: 0 }
  for (const e of events.value) m[e.status] = (m[e.status] || 0) + 1
  return m
})

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
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
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
    '1h': 3600000, '6h': 21600000, '24h': 86400000, '7d': 604800000, '30d': 2592000000,
  }
  const ms = map[timeRangePreset.value]
  if (ms) return { start_time: new Date(now.getTime() - ms).toISOString() }
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
      h('div', { class: 'severity-cell' }, [
        h('span', { class: `severity-bar severity-bar--${row.severity}` }),
        h(NTag, { type: getSeverityType(row.severity), size: 'small', round: true }, { default: () => row.severity.toUpperCase() }),
      ]),
  },
  {
    title: () => t('alert.alertName'),
    key: 'alert_name',
    ellipsis: { tooltip: true },
    minWidth: 180,
    render: (row: AlertEvent) => h('div', { class: 'name-cell' }, [
      row.severity === 'critical' && row.status === 'firing'
        ? h('span', { class: 'critical-pulse' })
        : null,
      h('a', {
        class: 'alert-link',
        onClick: () => router.push(`/alerts/events/${row.id}`),
      }, row.alert_name),
    ]),
  },
  {
    title: () => t('common.status'),
    key: 'status',
    width: 110,
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
    render: (row: AlertEvent) => h('span', { style: 'font-size:12px;color:var(--sre-text-secondary)' }, row.source || '-'),
  },
  {
    title: () => t('alert.firedAt'),
    key: 'fired_at',
    width: 160,
    render: (row: AlertEvent) => h('span', { style: 'font-size: 12px' }, formatTime(row.fired_at)),
  },
  {
    title: () => t('alert.duration'),
    key: 'duration',
    width: 90,
    render: (row: AlertEvent) => h('span', { style: 'font-size:12px;color:var(--sre-text-secondary);font-variant-numeric:tabular-nums' }, calcDuration(row)),
  },
  {
    title: '#',
    key: 'fire_count',
    width: 50,
    align: 'center' as const,
    render: (row: AlertEvent) => h('span', {
      style: `font-size:11px;font-weight:600;color:${row.fire_count > 5 ? '#e88080' : 'var(--sre-text-secondary)'}`,
    }, String(row.fire_count)),
  },
  {
    title: () => t('alert.ackedBy'),
    key: 'acked_by',
    width: 90,
    render: (row: AlertEvent) =>
      h('span', { style: 'font-size: 12px' }, row.acked_by_user?.display_name || '-'),
  },
  {
    title: () => t('alert.oncallUser'),
    key: 'oncall_user',
    width: 100,
    render: (row: AlertEvent) => {
      if (row.is_dispatched && row.oncall_user) {
        return h('span', { style: 'font-size:12px;color:#18a058' }, row.oncall_user.display_name || row.oncall_user.username)
      }
      return h('span', { style: 'font-size:11px;color:#666' }, '-')
    },
  },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 170,
    fixed: 'right' as const,
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
      return h(NSpace, { size: 3 }, { default: () => buttons })
    },
  },
]

async function fetchEvents() {
  if (groupedMode.value) {
    fetchGroups()
    return
  }
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
  } catch (err: any) { message.error(err.message) }
}

async function handleResolve(id: number) {
  try {
    await alertEventApi.resolve(id, { resolution: t('alert.manuallyResolved') })
    message.success(t('alert.alertResolved'))
    fetchEvents()
  } catch (err: any) { message.error(err.message) }
}

async function handleClose(id: number) {
  try {
    await alertEventApi.close(id)
    message.success(t('alert.alertClosed'))
    fetchEvents()
  } catch (err: any) { message.error(err.message) }
}

async function handleBatchAck() {
  if (!checkedRowKeys.value.length) return
  try {
    await alertEventApi.batchAcknowledge(checkedRowKeys.value)
    message.success(t('alert.batchAckSuccess'))
    checkedRowKeys.value = []
    fetchEvents()
  } catch (err: any) { message.error(err.message) }
}

async function handleBatchClose() {
  if (!checkedRowKeys.value.length) return
  try {
    await alertEventApi.batchClose(checkedRowKeys.value)
    message.success(t('alert.batchCloseSuccess'))
    checkedRowKeys.value = []
    fetchEvents()
  } catch (err: any) { message.error(err.message) }
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
  if (preset !== 'custom') customRange.value = null
  page.value = 1
  fetchEvents()
}

function handleCustomRange(val: [number, number] | null) {
  customRange.value = val
  if (val) { timeRangePreset.value = 'custom'; page.value = 1; fetchEvents() }
}

let refreshTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => { fetchEvents(); refreshTimer = setInterval(fetchEvents, 30000) })
onUnmounted(() => { if (refreshTimer) clearInterval(refreshTimer) })

const selectedText = computed(() => t('alert.selectedCount', { n: checkedRowKeys.value.length }))
function onCheckedRowKeysUpdate(keys: RowKey[]) { checkedRowKeys.value = keys as number[] }

const activeFiltersCount = computed(() => {
  let n = 0
  if (statusFilter.value.length) n++
  if (severityFilter.value.length) n++
  if (alertNameSearch.value) n++
  if (sourceFilter.value) n++
  if (timeRangePreset.value !== '24h') n++
  return n
})

// ===== Grouped view =====
const groupedMode = ref(false)
const groups = ref<AlertGroupItem[]>([])
const groupsLoading = ref(false)

async function fetchGroups() {
  groupsLoading.value = true
  try {
    const params: Record<string, string> = {}
    if (statusFilter.value.length) params.status = statusFilter.value.join(',')
    if (severityFilter.value.length) params.severity = severityFilter.value.join(',')
    const { data } = await alertGroupsApi.list(params)
    groups.value = data.data || []
  } catch (err: any) {
    message.error(err.message)
  } finally {
    groupsLoading.value = false
  }
}

function toggleGroupedMode() {
  groupedMode.value = !groupedMode.value
  if (groupedMode.value) fetchGroups()
}

function handleExportCSV() {
  const timeRange = getTimeRange()
  const params = new URLSearchParams()
  if (statusFilter.value.length) params.set('status', statusFilter.value.join(','))
  if (severityFilter.value.length) params.set('severity', severityFilter.value.join(','))
  if (alertNameSearch.value) params.set('alert_name', alertNameSearch.value)
  if (sourceFilter.value) params.set('source', sourceFilter.value)
  if (timeRange.start_time) params.set('start_time', timeRange.start_time)
  if (timeRange.end_time) params.set('end_time', timeRange.end_time)
  params.set('view_mode', viewMode.value)
  const url = `/api/v1/alert-events/export?${params.toString()}`
  const a = document.createElement('a')
  a.href = url
  a.download = `alert-events-${new Date().toISOString().slice(0, 10)}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}
</script>

<template>
  <div class="events-page">
    <PageHeader :title="t('alert.events')" :subtitle="t('alert.eventsSubtitle')">
      <template #actions>
        <n-text depth="3" style="font-size:13px">{{ t('alert.totalAlerts', { n: total }) }}</n-text>
        <n-button size="small" :secondary="groupedMode" :type="groupedMode ? 'primary' : 'default'" @click="toggleGroupedMode">
          <template #icon><n-icon :component="groupedMode ? ListOutline : LayersOutline" /></template>
          {{ groupedMode ? t('alert.flatView') : t('alert.groupedView') }}
        </n-button>
        <n-button size="small" @click="handleExportCSV">
          <template #icon><n-icon :component="DownloadOutline" /></template>
          {{ t('alert.exportCSV') }}
        </n-button>
        <n-button size="small" @click="fetchEvents" :loading="loading">
          <template #icon><n-icon :component="RefreshOutline" /></template>
          {{ t('common.refresh') }}
        </n-button>
      </template>
    </PageHeader>

    <!-- Toolbar: view mode + filter toggle + time presets -->
    <div class="toolbar">
      <n-radio-group :value="viewMode" @update:value="handleViewModeChange" size="small">
        <n-radio-button v-for="opt in viewModeOptions" :key="opt.value" :value="opt.value" :label="opt.label" />
      </n-radio-group>

      <div class="toolbar-sep" />

      <!-- Time presets -->
      <div class="time-presets">
        <button
          v-for="p in timePresets"
          :key="p.value"
          class="time-chip"
          :class="{ active: timeRangePreset === p.value }"
          @click="handleTimePreset(p.value)"
        >{{ p.label }}</button>
        <n-date-picker
          type="datetimerange"
          :value="customRange"
          clearable
          size="small"
          style="width:300px"
          @update:value="handleCustomRange"
        />
      </div>

      <div style="flex:1" />

      <!-- Filter toggle -->
      <n-button
        size="small"
        :secondary="showFilters"
        :type="activeFiltersCount > 0 ? 'primary' : 'default'"
        @click="showFilters = !showFilters"
      >
        <template #icon><n-icon :component="OptionsOutline" /></template>
        Filters
        <n-badge v-if="activeFiltersCount > 0" :value="activeFiltersCount" style="margin-left:4px" />
      </n-button>
    </div>

    <!-- Collapsible filter bar -->
    <div v-show="showFilters" class="filter-bar">
      <n-select
        v-model:value="statusFilter"
        :options="statusOptions"
        multiple
        :placeholder="t('common.status')"
        clearable
        style="width:220px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-select
        v-model:value="severityFilter"
        :options="severityOptions"
        multiple
        :placeholder="t('alert.severity')"
        clearable
        style="width:190px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-input
        v-model:value="alertNameSearch"
        :placeholder="t('alert.alertNameSearch')"
        clearable
        style="width:200px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-input
        v-model:value="sourceFilter"
        :placeholder="t('alert.sourceFilter')"
        clearable
        style="width:150px"
        @update:value="() => { page = 1; fetchEvents() }"
      />
      <n-button size="small" @click="resetFilters" :disabled="activeFiltersCount === 0">
        {{ t('alert.resetFilters') }}
      </n-button>
    </div>

    <!-- Status summary badges -->
    <div class="status-summary">
      <div v-for="(count, status) in statusCounts" :key="status" v-show="count > 0" class="status-pill" :class="`status-pill--${status}`">
        <span class="status-pill__dot" />
        <span>{{ status }}</span>
        <span class="status-pill__count">{{ count }}</span>
      </div>
    </div>

    <!-- Batch actions bar -->
    <transition name="slide-down">
      <div v-if="checkedRowKeys.length > 0" class="batch-bar">
        <n-icon :component="AlertCircleOutline" size="16" style="color:#637dff" />
        <span class="batch-bar__text">{{ selectedText }}</span>
        <div style="flex:1" />
        <n-button size="small" type="primary" @click="handleBatchAck">{{ t('alert.batchAck') }}</n-button>
        <n-button size="small" type="error" @click="handleBatchClose">{{ t('alert.batchClose') }}</n-button>
        <n-button size="small" quaternary @click="checkedRowKeys = []">{{ t('common.cancel') }}</n-button>
      </div>
    </transition>

    <!-- ===== Grouped View ===== -->
    <template v-if="groupedMode">
      <n-card :bordered="false" style="background:var(--sre-bg-card);border-radius:12px">
        <n-spin :show="groupsLoading">
          <n-empty v-if="!groupsLoading && groups.length === 0" :description="t('alert.noGroupedAlerts')" style="padding:40px 0" />
          <div v-else class="group-list">
            <div v-for="g in groups" :key="g.alert_name + g.source" class="group-card">
              <div class="group-header">
                <div class="group-name">
                  <n-tag v-if="g.severity_breakdown['critical'] > 0" type="error" size="small" round>critical ×{{ g.severity_breakdown['critical'] }}</n-tag>
                  <n-tag v-if="g.severity_breakdown['warning'] > 0" type="warning" size="small" round>warning ×{{ g.severity_breakdown['warning'] }}</n-tag>
                  <n-tag v-if="g.severity_breakdown['info'] > 0" type="info" size="small" round>info ×{{ g.severity_breakdown['info'] }}</n-tag>
                  <span class="group-alert-name">{{ g.alert_name }}</span>
                  <span v-if="g.source" class="group-source">@ {{ g.source }}</span>
                </div>
                <div class="group-meta">
                  <span class="group-count">{{ t('alert.groupTotal', { n: g.total_count }) }}</span>
                  <n-tag v-if="g.max_fire_count > 5" type="error" size="tiny">{{ t('alert.noisyAlert', { n: g.max_fire_count }) }}</n-tag>
                  <n-button size="tiny" quaternary @click="() => { alertNameSearch = g.alert_name; sourceFilter = g.source; groupedMode = false; page = 1; fetchEvents() }">
                    {{ t('alert.viewEvents') }}
                  </n-button>
                </div>
              </div>
              <div class="group-status-row">
                <span v-for="(cnt, st) in g.status_breakdown" :key="st" v-show="cnt > 0" class="status-chip" :class="`status-chip--${st}`">
                  {{ st }} {{ cnt }}
                </span>
                <span class="group-time">{{ t('alert.latestFired') }}: {{ formatTime(g.latest_fired_at) }}</span>
              </div>
            </div>
          </div>
        </n-spin>
      </n-card>
    </template>

    <!-- ===== Flat Events Table ===== -->
    <template v-else>
      <n-card :bordered="false" style="background:var(--sre-bg-card);border-radius:12px">
        <n-data-table
          :loading="loading"
          :columns="columns"
          :data="events"
          :row-key="(row: AlertEvent) => row.id"
          :row-class-name="severityRowClass"
          :checked-row-keys="checkedRowKeys"
          @update:checked-row-keys="onCheckedRowKeysUpdate"
          :bordered="false"
          scroll-x="1100"
          :pagination="{
            page, pageSize, itemCount: total,
            showSizePicker: true,
            pageSizes: [20, 50, 100],
            onChange: (p: number) => { page = p; fetchEvents() },
            onUpdatePageSize: (s: number) => { pageSize = s; page = 1; fetchEvents() },
          }"
        />
      </n-card>
    </template>
  </div>
</template>

<style scoped>
.events-page { max-width: 1440px; }

/* Page sections entrance */
.toolbar        { animation: sre-slide-up 0.22s var(--sre-ease-out) both; animation-delay: 0ms; }
.status-summary { animation: sre-slide-up 0.22s var(--sre-ease-out) both; animation-delay: 55ms; }
.batch-bar      { animation: sre-slide-up 0.18s var(--sre-ease-out) both; }
:deep(.n-card)  { animation: sre-slide-up 0.28s var(--sre-ease-out) both; animation-delay: 90ms; }

/* ===== Toolbar ===== */
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
  background: var(--sre-bg-card);
  border-radius: 10px;
  padding: 10px 16px;
}
.toolbar-sep {
  width: 1px;
  height: 20px;
  background: rgba(255,255,255,0.08);
}
.time-presets {
  display: flex;
  align-items: center;
  gap: 4px;
}
.time-chip {
  padding: 3px 10px;
  border-radius: 6px;
  border: 1px solid rgba(255,255,255,0.1);
  background: transparent;
  color: var(--sre-text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}
.time-chip:hover { background: rgba(255,255,255,0.06); color: var(--sre-text-primary); }
.time-chip.active {
  background: rgba(24,160,88,0.15);
  border-color: rgba(24,160,88,0.5);
  color: #18a058;
  font-weight: 600;
}

/* ===== Filter bar ===== */
.filter-bar {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
  align-items: center;
  background: var(--sre-bg-card);
  border-radius: 10px;
  padding: 12px 16px;
  border: 1px solid rgba(255,255,255,0.06);
}

/* ===== Status summary ===== */
.status-summary {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 10px;
  min-height: 24px;
}
.status-pill {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px 3px 7px;
  border-radius: 100px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(255,255,255,0.05);
  color: var(--sre-text-secondary);
  border: 1px solid rgba(255,255,255,0.08);
}
.status-pill__dot {
  width: 6px; height: 6px; border-radius: 50%;
}
.status-pill__count {
  font-weight: 700;
  margin-left: 3px;
}
.status-pill--firing { background: rgba(232,128,128,0.1); border-color: rgba(232,128,128,0.3); color: #e88080; }
.status-pill--firing .status-pill__dot { background: #e88080; animation: pulse-dot 1.5s ease-in-out infinite; }
.status-pill--acknowledged { background: rgba(242,201,125,0.1); border-color: rgba(242,201,125,0.3); color: #f2c97d; }
.status-pill--acknowledged .status-pill__dot { background: #f2c97d; }
.status-pill--assigned { background: rgba(112,192,232,0.1); border-color: rgba(112,192,232,0.3); color: #70c0e8; }
.status-pill--assigned .status-pill__dot { background: #70c0e8; }
.status-pill--resolved { background: rgba(24,160,88,0.1); border-color: rgba(24,160,88,0.3); color: #18a058; }
.status-pill--resolved .status-pill__dot { background: #18a058; }
.status-pill--closed .status-pill__dot { background: #666; }
.status-pill--silenced { background: rgba(168,85,247,0.1); border-color: rgba(168,85,247,0.3); color: #a855f7; }
.status-pill--silenced .status-pill__dot { background: #a855f7; }

@keyframes pulse-dot {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(232,128,128,0.5); }
  50% { opacity: 0.7; box-shadow: 0 0 0 3px rgba(232,128,128,0); }
}

/* ===== Batch bar ===== */
.batch-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(99,125,255,0.08);
  border: 1px solid rgba(99,125,255,0.2);
  border-radius: 10px;
  padding: 8px 16px;
  margin-bottom: 10px;
}
.batch-bar__text {
  font-size: 13px;
  color: #637dff;
  font-weight: 500;
}
.slide-down-enter-active, .slide-down-leave-active {
  transition: all 0.2s ease;
}
.slide-down-enter-from, .slide-down-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

/* ===== Table cell styles ===== */
:deep(.severity-cell) {
  display: flex;
  align-items: center;
  gap: 6px;
}
:deep(.severity-bar) {
  width: 3px;
  height: 20px;
  border-radius: 2px;
  flex-shrink: 0;
}
:deep(.severity-bar--critical) { background: #e88080; box-shadow: 0 0 4px rgba(232,128,128,0.6); }
:deep(.severity-bar--warning)  { background: #f2c97d; }
:deep(.severity-bar--info)     { background: #70c0e8; }

:deep(.name-cell) {
  display: flex;
  align-items: center;
  gap: 6px;
}
:deep(.critical-pulse) {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: #e88080;
  flex-shrink: 0;
  animation: pulse-dot 1.2s ease-in-out infinite;
}
:deep(.alert-link) {
  color: var(--sre-info);
  cursor: pointer;
  text-decoration: none;
  font-weight: 500;
}
:deep(.alert-link:hover) { text-decoration: underline; }

:deep(.row-critical td) {
  background-color: rgba(232,128,128,0.04) !important;
}
:deep(.row-warning td) {
  background-color: rgba(242,201,125,0.03) !important;
}

/* ===== Grouped view ===== */
.group-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.group-card {
  background: var(--sre-bg-elevated, rgba(255,255,255,0.04));
  border: 1px solid rgba(255,255,255,0.07);
  border-radius: 10px;
  padding: 12px 16px;
  transition: border-color 0.15s;
}
.group-card:hover { border-color: rgba(24,160,88,0.3); }
.group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.group-name {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.group-alert-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--sre-text-primary);
}
.group-source {
  font-size: 12px;
  color: var(--sre-text-secondary);
}
.group-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.group-count {
  font-size: 13px;
  font-weight: 600;
  color: var(--sre-text-primary);
}
.group-status-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.status-chip {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 100px;
  font-weight: 500;
  background: rgba(255,255,255,0.05);
  color: var(--sre-text-secondary);
}
.status-chip--firing    { background: rgba(232,128,128,0.15); color: #e88080; }
.status-chip--acknowledged { background: rgba(242,201,125,0.15); color: #f2c97d; }
.status-chip--assigned  { background: rgba(112,192,232,0.15); color: #70c0e8; }
.status-chip--resolved  { background: rgba(24,160,88,0.15); color: #18a058; }
.status-chip--silenced  { background: rgba(168,85,247,0.15); color: #a855f7; }
.group-time {
  margin-left: auto;
  font-size: 11px;
  color: var(--sre-text-secondary);
}
</style>
