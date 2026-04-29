<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { NSelect, NInput, NInputNumber, NButton, NDataTable, NEmpty, NSpin, NTag, NAlert } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { datasourceApi } from '@/api'
import type { DataSource, QueryResponse, LogEntry } from '@/types'
import TimeRangePicker from '@/components/time/TimeRangePicker.vue'
import RefreshPicker from '@/components/time/RefreshPicker.vue'
import { useTimeRange } from '@/composables/useTimeRange'

const { t } = useI18n()
const datasources = ref<DataSource[]>([])
const selectedDsId = ref<number | null>(null)

const {
  timeRange,
  isRelative,
  relativeDuration,
  autoRefreshInterval,
  setRelative,
  setAbsolute,
} = useTimeRange('1h')

const selectedDs = computed(() =>
  datasources.value.find(ds => ds.id === selectedDsId.value) || null
)

const isLogsMode = computed(() => selectedDs.value?.type === 'victorialogs')

const placeholderText = computed(() => {
  if (!selectedDs.value) return t('explore.enterExpression')
  switch (selectedDs.value.type) {
    case 'victorialogs': return t('explore.logQueryPlaceholder')
    case 'zabbix': return t('explore.zabbixPlaceholder')
    default: return t('explore.promqlPlaceholder')
  }
})

const stepAuto = computed(() => {
  const diff = (timeRange.value.end - timeRange.value.start) / 1000
  if (diff <= 3600) return '15s'
  if (diff <= 21600) return '1m'
  if (diff <= 86400) return '5m'
  return '15m'
})

// Query state
const expression = ref('')
const loading = ref(false)
const errorMsg = ref('')
const series = ref<QueryResponse['series']>([])
const logEntries = ref<LogEntry[]>([])
const logTotal = ref(0)
const logTruncated = ref(false)
const logLimit = ref(200)

const hasResults = computed(() => series.value.length > 0 || logEntries.value.length > 0)

// Metrics table
const metricsColumns = computed<DataTableColumns<any>>(() => [
  {
    title: t('explore.metricName') || 'Metric',
    key: 'name',
    ellipsis: { tooltip: true },
    render(row: any) { return (row.labels && row.labels.__name__) || '-' },
  },
  {
    title: t('explore.value') || 'Value',
    key: 'value',
    width: 140,
    render(row: any) {
      const v = row.value
      return typeof v === 'number' ? v.toFixed(4) : '-'
    },
  },
  {
    title: t('explore.labelsHeader') || 'Labels',
    key: 'labels',
    ellipsis: { tooltip: true },
    render(row: any) {
      const lbs: Record<string, string> = {}
      if (row.labels) {
        for (const k of Object.keys(row.labels)) {
          if (k !== '__name__') lbs[k] = row.labels[k]
        }
      }
      const parts = Object.entries(lbs).map(([k, v]) => `${k}=${v}`)
      return parts.length ? parts.join(' ') : '-'
    },
  },
])

const tableData = computed(() => {
  const rows: any[] = []
  let idx = 0
  for (const s of series.value) {
    const vals = s.values || []
    for (const v of vals) {
      rows.push({ labels: s.labels || {}, value: v.value, _key: idx++ })
    }
  }
  return rows
})

// Log columns — plain string returns, no h() usage
const logColumns = computed<DataTableColumns<LogEntry>>(() => [
  {
    title: t('explore.logTime') || 'Time',
    key: 'timestamp',
    width: 200,
    render(row: any) {
      const ts = row.timestamp
      if (!ts) return '-'
      try { return new Date(ts).toLocaleString() } catch { return String(ts) }
    },
  },
  {
    title: t('explore.logMessage') || 'Message',
    key: 'message',
    ellipsis: { tooltip: true },
    render(row: any) { return row.message || '-' },
  },
  {
    title: t('explore.logLabels') || 'Labels',
    key: 'labels',
    width: 400,
    render(row: any) {
      const labels = row.labels
      if (!labels || Object.keys(labels).length === 0) return '-'
      return Object.entries(labels).slice(0, 5).map(([k, v]) => `${k}=${v}`).join('  ')
    },
  },
])

// Execute query
async function executeQuery() {
  if (!selectedDsId.value || !expression.value.trim()) return

  loading.value = true
  errorMsg.value = ''
  series.value = []
  logEntries.value = []

  try {
    const tr = timeRange.value
    if (isLogsMode.value) {
      const res = await datasourceApi.logQuery(selectedDsId.value, {
        expression: expression.value,
        start: Math.floor(tr.start / 1000),
        end: Math.floor(tr.end / 1000),
        limit: logLimit.value,
      })
      const data = res.data.data
      logEntries.value = data.entries || []
      logTotal.value = data.total || 0
      logTruncated.value = data.truncated || false
    } else {
      const res = await datasourceApi.rangeQuery(selectedDsId.value, {
        expression: expression.value,
        start: Math.floor(tr.start / 1000),
        end: Math.floor(tr.end / 1000),
        step: stepAuto.value,
      })
      const data = res.data.data
      series.value = data.series || []
    }
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.message || err?.message || t('explore.queryFailed')
  } finally {
    loading.value = false
  }
}

function handleKeyup(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    executeQuery()
  }
}

watch(selectedDsId, () => {
  expression.value = ''
  series.value = []
  logEntries.value = []
  errorMsg.value = ''
})

// Auto-refresh
let refreshTimer: ReturnType<typeof setInterval> | null = null
watch(autoRefreshInterval, (val) => {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
  if (val && val > 0) {
    refreshTimer = setInterval(executeQuery, val * 1000)
  }
})

const datasourceOptions = computed(() =>
  datasources.value.map(ds => ({ label: ds.name, value: ds.id }))
)

async function fetchDatasources() {
  try {
    const res = await datasourceApi.list({ page: 1, page_size: 100 })
    const list = res.data?.data?.list
    datasources.value = (Array.isArray(list) ? list : []).filter((ds: any) => ds.is_enabled)
    if (datasources.value.length > 0 && !selectedDsId.value) {
      selectedDsId.value = datasources.value[0].id
    }
  } catch {
    // ignore
  }
}

onMounted(fetchDatasources)
</script>

<template>
  <div class="explore-page">
    <!-- Header -->
    <div class="explore-header">
      <div class="header-left">
        <h2 class="page-title">{{ t('explore.title') }}</h2>
        <span class="page-subtitle">{{ t('explore.subtitle') }}</span>
      </div>
      <div class="header-right">
        <TimeRangePicker
          :time-range="timeRange"
          :is-relative="isRelative"
          :relative-duration="relativeDuration"
          @set-relative="setRelative"
          @set-absolute="setAbsolute"
        />
        <RefreshPicker
          :value="autoRefreshInterval"
          @update:value="(v: number | null) => autoRefreshInterval = v"
        />
      </div>
    </div>

    <!-- Datasource selector -->
    <div class="ds-select-row">
      <NSelect
        v-model:value="selectedDsId"
        :options="datasourceOptions"
        :placeholder="t('explore.selectDatasource')"
        filterable
        style="width: 320px"
        size="small"
      />
      <span v-if="selectedDs" class="ds-type-badge">
        {{ selectedDs.type }}
      </span>
    </div>

    <!-- No datasource yet -->
    <div v-if="!selectedDsId" class="empty-state">
      <NEmpty :description="t('explore.selectDatasource')" />
    </div>

    <!-- Query bar -->
    <div v-if="selectedDsId" class="query-bar">
      <NInput
        v-model:value="expression"
        type="textarea"
        :placeholder="placeholderText"
        size="small"
        :autosize="{ minRows: 1, maxRows: 6 }"
        style="flex: 1"
        @keyup="handleKeyup"
      />
      <NInputNumber
        v-if="isLogsMode"
        v-model:value="logLimit"
        :min="10"
        :max="10000"
        size="small"
        style="width: 110px"
        :placeholder="t('explore.limit')"
      />
      <NButton
        type="primary"
        size="small"
        :loading="loading"
        :disabled="!expression.trim()"
        @click="executeQuery"
      >
        {{ t('explore.runQuery') }}
      </NButton>
      <span class="query-hint">Ctrl+Enter</span>
    </div>

    <!-- Error -->
    <NAlert
      v-if="errorMsg"
      type="error"
      :show-icon="true"
      closable
      style="margin: 12px 0"
      @close="errorMsg = ''"
    >
      {{ errorMsg }}
    </NAlert>

    <!-- Results: Metrics table -->
    <div v-if="hasResults && !isLogsMode" class="results-section">
      <div class="results-header">
        <span class="results-count">
          {{ t('explore.table') }} — {{ tableData.length }} {{ t('explore.entries') || 'rows' }}
        </span>
      </div>
      <NDataTable
        :columns="metricsColumns"
        :data="tableData"
        :max-height="600"
        :row-key="(_row: any, index: number) => index"
        size="small"
        striped
      />
    </div>

    <!-- Results: Logs table -->
    <div v-if="isLogsMode && logEntries.length > 0" class="results-section">
      <div class="results-header">
        <span class="results-count">
          {{ t('explore.showing') }} {{ logEntries.length }}
          <template v-if="logTotal > 0"> / {{ logTotal }}</template>
          {{ t('explore.entries') }}
          <NTag v-if="logTruncated" type="warning" size="small" style="margin-left: 8px">
            {{ t('explore.truncated') }}
          </NTag>
        </span>
      </div>
      <NDataTable
        :columns="logColumns"
        :data="logEntries"
        :max-height="600"
        :row-key="(_row: any, index: number) => index"
        :scrollbar-props="{ trigger: 'hover' }"
        size="small"
        striped
      />
    </div>

    <!-- Empty log result -->
    <div v-if="isLogsMode && !loading && !errorMsg && expression.trim() && logEntries.length === 0" class="empty-state">
      <NEmpty :description="t('explore.logEmptyDesc')" />
    </div>

    <!-- No metrics results yet -->
    <div v-if="!isLogsMode && !loading && !errorMsg && expression.trim() && !hasResults" class="empty-state">
      <NEmpty :description="t('explore.logEmptyDesc') || 'No results'" />
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-wrap">
      <NSpin size="medium" />
    </div>
  </div>
</template>

<style scoped>
.explore-page {
  max-width: 1600px;
  padding: 20px;
}
.explore-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.header-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.page-title {
  font-size: 22px;
  font-weight: 600;
  margin: 0;
}
.page-subtitle {
  font-size: 13px;
  color: var(--sre-text-secondary);
}
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.ds-select-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.ds-type-badge {
  font-size: 11px;
  color: var(--sre-text-tertiary);
  background: var(--sre-bg-hover, #f0f0f0);
  padding: 2px 8px;
  border-radius: 4px;
  font-family: monospace;
}
.query-bar {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px 16px;
  background: var(--sre-bg-card);
  border-radius: 8px;
  border: 1px solid var(--sre-border);
}
.query-hint {
  font-size: 11px;
  color: var(--sre-text-tertiary);
  align-self: center;
  white-space: nowrap;
}
.results-section {
  margin-top: 16px;
  background: var(--sre-bg-card);
  border-radius: 12px;
  padding: 16px;
}
.results-header {
  margin-bottom: 12px;
}
.results-count {
  font-size: 13px;
  color: var(--sre-text-secondary);
}
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}
.loading-wrap {
  display: flex;
  justify-content: center;
  padding: 40px;
}
</style>
