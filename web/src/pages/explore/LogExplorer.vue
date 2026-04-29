<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { NDataTable, NEmpty, NSpin, NTag, NScrollbar } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { datasourceApi } from '@/api'
import type { DataSource, LogEntry } from '@/types'
import { useTimeRange } from '@/composables/useTimeRange'
import TimeRangePicker from '@/components/time/TimeRangePicker.vue'
import RefreshPicker from '@/components/time/RefreshPicker.vue'

const { t } = useI18n()
const datasources = ref<DataSource[]>([])
const selectedDsId = ref<number | null>(null)
const expression = ref('')
const limit = ref(200)
const loading = ref(false)
const logEntries = ref<LogEntry[]>([])
const truncated = ref(false)
const errorMsg = ref('')

const {
  timeRange,
  isRelative,
  relativeDuration,
  autoRefreshInterval,
  setRelative,
  setAbsolute,
} = useTimeRange('1h')

const vlDatasources = computed(() =>
  datasources.value.filter(ds => ds.type === 'victorialogs' && ds.is_enabled)
)

const columns: DataTableColumns<LogEntry> = [
  {
    title: 'Time',
    key: 'timestamp',
    width: 200,
    render(row) {
      const ts = row.timestamp
      if (!ts) return '-'
      try {
        const d = new Date(ts)
        return d.toLocaleString()
      } catch {
        return ts
      }
    },
  },
  {
    title: 'Message',
    key: 'message',
    ellipsis: { tooltip: true },
    render(row) {
      return row.message || '-'
    },
  },
  {
    title: 'Labels',
    key: 'labels',
    width: 400,
    render(row) {
      const labels = row.labels
      if (!labels || Object.keys(labels).length === 0) return '-'
      const entries = Object.entries(labels).slice(0, 5)
      return entries.map(([k, v]) =>
        h(NTag, { size: 'small', bordered: false, style: 'margin: 2px' }, () => `${k}=${v}`)
      )
    },
  },
]

import { h } from 'vue'

async function fetchDatasources() {
  try {
    const res = await datasourceApi.list({ page: 1, page_size: 100, type: 'victorialogs' })
    datasources.value = (res.data.data.list || []).filter((ds: any) => ds.is_enabled)
    if (vlDatasources.value.length > 0 && !selectedDsId.value) {
      selectedDsId.value = vlDatasources.value[0].id
    }
  } catch {
    // ignore
  }
}

async function executeQuery() {
  if (!selectedDsId.value || !expression.value.trim()) return

  loading.value = true
  errorMsg.value = ''
  logEntries.value = []
  truncated.value = false

  try {
    const tr = timeRange.value
    const res = await datasourceApi.logQuery(selectedDsId.value, {
      expression: expression.value,
      start: Math.floor(tr.start / 1000),
      end: Math.floor(tr.end / 1000),
      limit: limit.value,
    })
    const data = res.data.data
    logEntries.value = data.entries || []
    truncated.value = data.truncated || false
  } catch (err: any) {
    errorMsg.value = err?.message || 'Query failed'
  } finally {
    loading.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    executeQuery()
  }
}

onMounted(fetchDatasources)
</script>

<template>
  <div class="log-explorer-page">
    <div class="explore-header">
      <div class="header-left">
        <h2 class="page-title">{{ t('logExplorer.title') || 'Log Explorer' }}</h2>
        <span class="page-subtitle">{{ t('logExplorer.subtitle') || 'Query logs from VictoriaLogs' }}</span>
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

    <!-- Query Bar -->
    <div class="query-bar">
      <n-select
        v-model:value="selectedDsId"
        :options="vlDatasources.map(ds => ({ label: ds.name, value: ds.id }))"
        :placeholder="t('logExplorer.selectDatasource') || 'Select VictoriaLogs datasource'"
        style="width: 240px"
        size="small"
      />
      <n-input
        v-model:value="expression"
        :placeholder="t('logExplorer.queryPlaceholder') || 'Enter LogsQL query (e.g. level:error _time:1h)'"
        size="small"
        style="flex: 1"
        @keydown="handleKeydown"
      />
      <n-input-number
        v-model:value="limit"
        :min="10"
        :max="10000"
        size="small"
        style="width: 120px"
        :placeholder="t('logExplorer.limit') || 'Limit'"
      />
      <n-button
        type="primary"
        size="small"
        :loading="loading"
        :disabled="!selectedDsId || !expression.trim()"
        @click="executeQuery"
      >
        {{ t('logExplorer.runQuery') || 'Run Query' }}
      </n-button>
    </div>

    <!-- Error -->
    <n-alert v-if="errorMsg" type="error" :show-icon="true" closable style="margin: 12px 0" @close="errorMsg = ''">
      {{ errorMsg }}
    </n-alert>

    <!-- Results -->
    <div class="results-section">
      <div class="results-header" v-if="logEntries.length > 0">
        <span class="results-count">
          {{ t('logExplorer.showing') || 'Showing' }} {{ logEntries.length }} {{ t('logExplorer.entries') || 'entries' }}
          <n-tag v-if="truncated" type="warning" size="small" style="margin-left: 8px">
            {{ t('logExplorer.truncated') || 'Truncated' }}
          </n-tag>
        </span>
      </div>

      <n-data-table
        v-if="logEntries.length > 0"
        :columns="columns"
        :data="logEntries"
        :row-key="(row: LogEntry) => row.timestamp + row.message"
        :max-height="600"
        :scrollbar-props="{ trigger: 'hover' }"
        size="small"
        striped
      />

      <div v-else-if="!loading && !errorMsg" class="empty-state">
        <n-empty :description="t('logExplorer.emptyDesc') || 'Enter a LogsQL query and click Run Query to view logs'" />
      </div>

      <div v-if="loading" class="loading-overlay">
        <n-spin size="medium" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-explorer-page {
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
  color: #666;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.query-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: var(--n-card-color, #fff);
  border-radius: 8px;
  border: 1px solid var(--n-border-color, #eee);
}
.results-section {
  margin-top: 16px;
  background: var(--n-card-color, #fff);
  border-radius: 12px;
  padding: 16px;
  min-height: 200px;
  position: relative;
}
.results-header {
  margin-bottom: 12px;
}
.results-count {
  font-size: 13px;
  color: #666;
}
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}
.loading-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.7);
  border-radius: 12px;
  z-index: 10;
}
</style>
