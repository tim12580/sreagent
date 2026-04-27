<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { datasourceApi } from '@/api'
import type { DataSource, QueryResponse } from '@/types'

const message = useMessage()
const { t } = useI18n()

const datasources = ref<DataSource[]>([])
const selectedDsId = ref<number | null>(null)
const expression = ref('')
const queryTime = ref(0)
const loading = ref(false)
const queryResult = ref<QueryResponse | null>(null)
const queryError = ref('')

const timeOptions = [
  { label: 'now', value: 0 },
  { label: '5m ago', value: -300 },
  { label: '15m ago', value: -900 },
  { label: '30m ago', value: -1800 },
  { label: '1h ago', value: -3600 },
  { label: '3h ago', value: -10800 },
  { label: '6h ago', value: -21600 },
  { label: '12h ago', value: -43200 },
  { label: '1d ago', value: -86400 },
]

async function fetchDatasources() {
  try {
    const res = await datasourceApi.list({ page: 1, page_size: 100 })
    datasources.value = (res.data.data.list || []).filter((ds: DataSource) => ds.is_enabled)
  } catch (err: any) {
    message.error(err.message || 'Failed to load datasources')
  }
}

async function handleQuery() {
  if (!selectedDsId.value) {
    message.warning(t('datasource.selectDatasource'))
    return
  }
  if (!expression.value.trim()) {
    message.warning(t('datasource.queryExpression'))
    return
  }

  loading.value = true
  queryResult.value = null
  queryError.value = ''
  try {
    const res = await datasourceApi.query(selectedDsId.value, {
      expression: expression.value,
      time: queryTime.value === 0 ? 0 : Date.now() / 1000 + queryTime.value,
    })
    queryResult.value = res.data.data
  } catch (err: any) {
    queryError.value = err.message || 'Query failed'
  } finally {
    loading.value = false
  }
}

function formatTimestamp(ts: number) {
  return new Date(ts * 1000).toLocaleString()
}

onMounted(fetchDatasources)
</script>

<template>
  <div class="query-page">
    <h2 class="page-title">{{ t('datasource.queryTitle') }}</h2>
    <p class="page-subtitle">{{ t('datasource.querySubtitle') }}</p>

    <div class="query-card">
      <div class="query-row">
        <div class="query-field">
          <label>{{ t('datasource.selectDatasource') }}</label>
          <select v-model="selectedDsId" class="form-select">
            <option :value="null" disabled>{{ t('datasource.selectDatasource') }}</option>
            <option v-for="ds in datasources" :key="ds.id" :value="ds.id">
              {{ ds.name }} ({{ ds.type }})
            </option>
          </select>
        </div>
        <div class="query-field">
          <label>{{ t('datasource.queryTime') }}</label>
          <select v-model="queryTime" class="form-select">
            <option v-for="opt in timeOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </div>
      </div>

      <div class="query-field">
        <label>{{ t('datasource.queryExpression') }}</label>
        <textarea
          v-model="expression"
          class="form-textarea"
          :placeholder="t('datasource.queryPlaceholder')"
          rows="4"
          @keyup.ctrl.enter="handleQuery"
        ></textarea>
      </div>

      <button
        class="btn-primary"
        :disabled="loading || !selectedDsId || !expression.trim()"
        @click="handleQuery"
      >
        {{ loading ? '...' : t('datasource.executeQuery') }}
      </button>
    </div>

    <div v-if="queryError" class="error-box">
      {{ queryError }}
    </div>

    <div v-if="queryResult" class="query-card" style="margin-top: 16px">
      <div class="result-header">
        <span>{{ t('datasource.queryResult') }}</span>
        <span class="tag">{{ queryResult.result_type }}</span>
        <span class="tag">{{ queryResult.series?.length ?? queryResult.raw_count ?? 0 }} series</span>
      </div>

      <div v-if="!queryResult.series || queryResult.series.length === 0" class="empty-box">
        {{ t('datasource.queryNoResult') }}
      </div>

      <table v-else-if="queryResult.result_type === 'vector' || queryResult.result_type === 'matrix'" class="result-table">
        <thead>
          <tr>
            <th>Labels</th>
            <th>Values</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(s, i) in queryResult.series" :key="i">
            <td>{{ Object.entries(s.labels).map(([k, v]) => `${k}=${v}`).join(', ') }}</td>
            <td>{{ s.values.map(v => `${formatTimestamp(v.ts)}: ${v.value}`).join('\n') }}</td>
          </tr>
        </tbody>
      </table>

      <pre v-else class="json-block">{{ JSON.stringify(queryResult.series, null, 2) }}</pre>
    </div>
  </div>
</template>

<style scoped>
.query-page { max-width: 1400px; padding: 20px; }
.page-title { font-size: 22px; font-weight: 600; margin: 0 0 4px; }
.page-subtitle { font-size: 13px; color: #666; margin: 0 0 20px; }
.query-card { background: #fff; border-radius: 12px; padding: 20px; }
.query-row { display: flex; gap: 12px; margin-bottom: 16px; }
.query-field { flex: 1; margin-bottom: 12px; }
.query-field label { display: block; margin-bottom: 4px; font-size: 13px; color: #666; }
.form-select { width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; }
.form-textarea { width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; resize: vertical; box-sizing: border-box; }
.btn-primary { padding: 8px 20px; background: #18a058; color: #fff; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.error-box { margin-top: 16px; padding: 12px; background: #fff2f0; border: 1px solid #ffccc7; border-radius: 6px; color: #cf1322; }
.result-header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; font-weight: 600; }
.tag { font-size: 12px; padding: 2px 8px; background: #f0f0f0; border-radius: 4px; font-weight: normal; }
.result-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.result-table th, .result-table td { padding: 8px; border: 1px solid #eee; text-align: left; }
.result-table th { background: #fafafa; }
.empty-box { padding: 40px; text-align: center; color: #999; }
.json-block { background: #f5f5f5; padding: 12px; border-radius: 6px; overflow: auto; font-size: 13px; }
</style>
