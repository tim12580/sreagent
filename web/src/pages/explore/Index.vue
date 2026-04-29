<script setup lang="ts">
import { ref, onMounted, computed, watch, h, shallowRef } from 'vue'
import { NTabs, NTabPane, NDataTable, NEmpty, NSpin, NTag, NAlert, NButton, NSelect, NInputNumber } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { TooltipComponent, LegendComponent, GridComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import { datasourceApi } from '@/api'
import type { DataSource, QueryResponse, LogEntry } from '@/types'
import TimeRangePicker from '@/components/time/TimeRangePicker.vue'
import RefreshPicker from '@/components/time/RefreshPicker.vue'
import { useTimeRange } from '@/composables/useTimeRange'

use([CanvasRenderer, LineChart, TooltipComponent, LegendComponent, GridComponent])

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

// --- Derived state ---
const selectedDs = computed(() =>
  datasources.value.find(ds => ds.id === selectedDsId.value) || null
)

const isLogsMode = computed(() => selectedDs.value?.type === 'victorialogs')
const isZabbixMode = computed(() => selectedDs.value?.type === 'zabbix')

const queryPlaceholder = computed(() => {
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

// --- Query state ---
const expression = ref('')
const loading = ref(false)
const errorMsg = ref('')
const resultType = ref<'vector' | 'matrix' | 'logs' | null>(null)
const series = ref<QueryResponse['series']>([])
const logEntries = ref<LogEntry[]>([])
const logTotal = ref(0)
const logTruncated = ref(false)
const logLimit = ref(200)
const resultTab = ref('chart')

// --- Chart data derived from series ---
const hasResults = computed(() => series.value.length > 0 || logEntries.value.length > 0)

const chartOption = computed(() => {
  if (!hasResults.value || resultType.value === 'logs') return null

  const xData: string[] = []
  const seriesList: any[] = []
  const seen = new Map<string, boolean>()

  for (const s of series.value) {
    const labelStr = Object.entries(s.labels)
      .filter(([k]) => k !== '__name__')
      .map(([k, v]) => `${k}=${v}`)
      .join(',')
    const name = labelStr || s.labels.__name__ || 'value'

    if (resultType.value === 'vector') {
      const displayName = name || 'value'
      if (!seen.has(displayName)) {
        seen.set(displayName, true)
        seriesList.push({
          name: displayName,
          type: 'bar',
          data: s.values.map(v => v.value),
        })
        xData.push(displayName)
      }
    } else {
      if (!seen.has(name)) {
        seen.set(name, true)
        seriesList.push({
          name,
          type: 'line',
          smooth: true,
          symbol: 'none',
          data: [] as [string, number][],
        })
      }
      const target = seriesList.find(sl => sl.name === name)
      if (target) {
        for (const v of s.values) {
          const ts = new Date(v.ts * 1000).toLocaleTimeString()
          target.data.push([ts, v.value])
        }
      }
    }
  }

  // For matrix results, collect all timestamps
  if (resultType.value === 'matrix') {
    const allTimes = new Set<string>()
    for (const sl of seriesList) {
      for (const d of sl.data) {
        allTimes.add(d[0])
      }
    }
    // Sort timestamps
    const sorted = Array.from(allTimes).sort()
    // Rebuild each series aligned to sorted timestamps
    for (const sl of seriesList) {
      const timeMap = new Map(sl.data.map((d: [string, number]) => [d[0], d[1]]))
      sl.data = sorted.map(t => timeMap.get(t) ?? null)
    }
    return {
      tooltip: { trigger: 'axis' as const },
      legend: { type: 'scroll' as const, bottom: 0, textStyle: { fontSize: 11 } },
      grid: { left: 60, right: 20, top: 20, bottom: 50 },
      xAxis: { type: 'category' as const, data: sorted },
      yAxis: { type: 'value' as const },
      series: seriesList,
    }
  }

  return {
    tooltip: { trigger: 'axis' as const },
    legend: { show: false },
    grid: { left: 60, right: 20, top: 20, bottom: 30 },
    xAxis: { type: 'category' as const, data: xData },
    yAxis: { type: 'value' as const },
    series: seriesList,
  }
})

// --- Metrics columns ---
const metricsColumns: DataTableColumns<{ labels: Record<string, string>; value: number }> = [
  { title: t('explore.metricName') || 'Metric', key: 'name', ellipsis: { tooltip: true },
    render(row) { return row.labels.__name__ || '-' },
  },
  { title: t('explore.value') || 'Value', key: 'value', width: 140,
    render(row) { return row.value?.toFixed(4) || '-' },
  },
  { title: t('explore.labelsHeader') || 'Labels', key: 'labels', ellipsis: { tooltip: true },
    render(row) {
      const lbs = { ...row.labels }
      delete lbs.__name__
      return Object.entries(lbs).map(([k, v]) => `${k}=${v}`).join(' ') || '-'
    },
  },
]

const tableData = computed(() => {
  const rows: { labels: Record<string, string>; value: number }[] = []
  for (const s of series.value) {
    for (const v of s.values) {
      rows.push({ labels: s.labels, value: v.value })
    }
  }
  return rows
})

// --- Log columns ---
const logColumns: DataTableColumns<LogEntry> = [
  {
    title: t('explore.logTime') || 'Time',
    key: 'timestamp',
    width: 200,
    render(row) {
      const ts = row.timestamp
      if (!ts) return '-'
      try { return new Date(ts).toLocaleString() } catch { return ts }
    },
  },
  {
    title: t('explore.logMessage') || 'Message',
    key: 'message',
    ellipsis: { tooltip: true },
    render(row) { return row.message || '-' },
  },
  {
    title: t('explore.logLabels') || 'Labels',
    key: 'labels',
    width: 400,
    render(row) {
      const labels = row.labels
      if (!labels || Object.keys(labels).length === 0) return '-'
      return Object.entries(labels).slice(0, 5).map(([k, v]) =>
        h(NTag, { size: 'small', bordered: false, style: 'margin: 2px' }, () => `${k}=${v}`)
      )
    },
  },
]

// --- Execute query ---
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
      resultType.value = 'logs'
    } else {
      const res = await datasourceApi.rangeQuery(selectedDsId.value, {
        expression: expression.value,
        start: Math.floor(tr.start / 1000),
        end: Math.floor(tr.end / 1000),
        step: stepAuto.value,
      })
      const data = res.data.data
      resultType.value = data.result_type
      series.value = data.series || []
    }
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.message || err?.message || t('explore.queryFailed')
  } finally {
    loading.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
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

async function fetchDatasources() {
  try {
    const res = await datasourceApi.list({ page: 1, page_size: 100 })
    datasources.value = (res.data.data.list || []).filter((ds: any) => ds.is_enabled)
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
      <n-select
        v-model:value="selectedDsId"
        :options="datasources.map(ds => ({
          label: ds.name,
          value: ds.id,
          tag: ds.type,
        }))"
        :placeholder="t('explore.selectDatasource')"
        filterable
        style="width: 320px"
        size="small"
      />
      <span v-if="selectedDs" class="ds-type-badge">
        {{ selectedDs.type }}
      </span>
    </div>

    <!-- No datasource -->
    <div v-if="!selectedDsId" class="empty-state">
      <n-empty :description="t('explore.selectDatasource')" />
    </div>

    <!-- Query bar -->
    <div v-if="selectedDsId" class="query-bar">
      <n-input
        v-model:value="expression"
        type="textarea"
        :placeholder="queryPlaceholder"
        size="small"
        :autosize="{ minRows: 1, maxRows: 6 }"
        style="flex: 1"
        @keydown="handleKeydown"
      />
      <n-input-number
        v-if="isLogsMode"
        v-model:value="logLimit"
        :min="10"
        :max="10000"
        size="small"
        style="width: 110px"
        :placeholder="t('explore.limit')"
      />
      <n-button
        type="primary"
        size="small"
        :loading="loading"
        :disabled="!expression.trim()"
        @click="executeQuery"
      >
        {{ t('explore.runQuery') }}
      </n-button>
      <span class="query-hint">Ctrl+Enter</span>
    </div>

    <!-- Error -->
    <n-alert v-if="errorMsg" type="error" :show-icon="true" closable style="margin: 12px 0" @close="errorMsg = ''">
      {{ errorMsg }}
    </n-alert>

    <!-- Results: METRICS -->
    <template v-if="selectedDsId && !isLogsMode && hasResults">
      <div class="results-section">
        <NTabs v-model:value="resultTab" type="line" size="small">
          <NTabPane name="chart" :tab="t('explore.chart')">
            <div v-if="chartOption" class="chart-container">
              <VChart
                :option="chartOption"
                autoresize
                style="height: 400px"
              />
            </div>
          </NTabPane>
          <NTabPane name="table" :tab="t('explore.table')">
            <NDataTable
              :columns="metricsColumns"
              :data="tableData"
              :max-height="400"
              :row-key="(row: any, idx: number) => idx"
              size="small"
              striped
            />
          </NTabPane>
        </NTabs>
      </div>
    </template>

    <!-- Results: LOGS -->
    <template v-if="selectedDsId && isLogsMode">
      <div v-if="logEntries.length > 0" class="results-section">
        <div class="results-header">
          <span class="results-count">
            {{ t('explore.showing') }} {{ logEntries.length }}
            <template v-if="logTotal > 0"> / {{ logTotal }}</template>
            {{ t('explore.entries') }}
            <n-tag v-if="logTruncated" type="warning" size="small" style="margin-left: 8px">
              {{ t('explore.truncated') }}
            </n-tag>
          </span>
        </div>

        <NDataTable
          :columns="logColumns"
          :data="logEntries"
          :row-key="(row: LogEntry) => row.timestamp + row.message"
          :max-height="600"
          :scrollbar-props="{ trigger: 'hover' }"
          size="small"
          striped
        />
      </div>

      <div v-else-if="!loading && !errorMsg && expression.trim()" class="empty-state">
        <n-empty :description="t('explore.logEmptyDesc')" />
      </div>
    </template>

    <div v-if="loading" style="display: flex; justify-content: center; padding: 40px">
      <n-spin size="medium" />
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
.chart-container {
  min-height: 400px;
}
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}
</style>
