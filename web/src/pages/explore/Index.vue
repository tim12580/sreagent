<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { datasourceApi } from '@/api'
import type { DataSource } from '@/types'

const { t } = useI18n()

// --- data ---
const datasources = ref<DataSource[]>([])
const selectedDsId = ref<number | null>(null)
const expression = ref('')
const loading = ref(false)
const errorMsg = ref('')
const logEntries = ref<any[]>([])
const metricRows = ref<any[]>([])
const logTotal = ref(0)
const logTruncated = ref(false)
const logLimit = ref(200)

const selectedDs = computed(() => datasources.value.find(d => d.id === selectedDsId.value))
const isLogs = computed(() => selectedDs.value?.type === 'victorialogs')

// --- time ---
const now = ref(Date.now())
const rangeH = ref(1)
const timeStart = computed(() => now.value - rangeH.value * 3600000)
const timeEnd = computed(() => now.value)

// --- actions ---
async function loadDs() {
  try {
    const res = await datasourceApi.list({ page: 1, page_size: 100 })
    const list = res.data?.data?.list
    datasources.value = (Array.isArray(list) ? list : []).filter((d: any) => d.is_enabled)
    if (datasources.value.length && !selectedDsId.value) selectedDsId.value = datasources.value[0].id
  } catch { /* ignore */ }
}

async function run() {
  if (!selectedDsId.value || !expression.value.trim()) return
  loading.value = true; errorMsg.value = ''; metricRows.value = []; logEntries.value = []
  try {
    if (isLogs.value) {
      const res = await datasourceApi.logQuery(selectedDsId.value, {
        expression: expression.value,
        start: Math.floor(timeStart.value / 1000),
        end: Math.floor(timeEnd.value / 1000),
        limit: logLimit.value,
      })
      const data = res.data.data
      logEntries.value = (data.entries || []).map((e: any, i: number) => ({ ...e, _key: i }))
      logTotal.value = data.total || 0
      logTruncated.value = data.truncated || false
    } else {
      const diff = (timeEnd.value - timeStart.value) / 1000
      const step = diff <= 3600 ? '15s' : diff <= 21600 ? '1m' : diff <= 86400 ? '5m' : '15m'
      const res = await datasourceApi.rangeQuery(selectedDsId.value, {
        expression: expression.value,
        start: Math.floor(timeStart.value / 1000),
        end: Math.floor(timeEnd.value / 1000),
        step,
      })
      const series = res.data.data?.series || []
      const rows: any[] = []
      let idx = 0
      for (const s of series) {
        for (const v of (s.values || [])) {
          rows.push({ name: (s.labels?.__name__) || '-', value: typeof v.value === 'number' ? v.value.toFixed(4) : '-', labels: s.labels, _key: idx++ })
        }
      }
      metricRows.value = rows
    }
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.message || e?.message || 'Query failed'
  } finally { loading.value = false }
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); run() }
}

watch(selectedDsId, () => { expression.value = ''; metricRows.value = []; logEntries.value = []; errorMsg.value = '' })

let timer: any = null
watch(() => rangeH.value, () => { now.value = Date.now() }) // no auto-refresh for now

onMounted(loadDs)
</script>

<template>
  <div style="max-width:1600px;padding:20px;">
    <!-- header -->
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px;">
      <div>
        <h2 style="font-size:22px;font-weight:600;margin:0;">{{ t('explore.title') }}</h2>
        <span style="font-size:13px;color:var(--sre-text-secondary)">{{ t('explore.subtitle') }}</span>
      </div>
      <div style="display:flex;align-items:center;gap:12px;">
        <select v-model.number="rangeH" style="padding:4px 8px;border-radius:4px;border:1px solid var(--sre-border);background:var(--sre-bg-card);color:var(--sre-text-primary);font-size:12px;">
          <option :value="1">Last 1 hour</option>
          <option :value="6">Last 6 hours</option>
          <option :value="24">Last 24 hours</option>
          <option :value="168">Last 7 days</option>
        </select>
      </div>
    </div>

    <!-- datasource selector -->
    <div style="display:flex;align-items:center;gap:8px;margin-bottom:12px;">
      <select v-model="selectedDsId" style="width:320px;padding:6px 10px;border-radius:6px;border:1px solid var(--sre-border);background:var(--sre-bg-card);color:var(--sre-text-primary);font-size:13px;">
        <option :value="null" disabled>{{ t('explore.selectDatasource') }}</option>
        <option v-for="ds in datasources" :key="ds.id" :value="ds.id">{{ ds.name }}</option>
      </select>
      <span v-if="selectedDs" style="font-size:11px;background:var(--sre-bg-hover,#f0f0f0);padding:2px 8px;border-radius:4px;font-family:monospace;color:var(--sre-text-tertiary);">{{ selectedDs.type }}</span>
    </div>

    <!-- no datasource -->
    <div v-if="!selectedDsId" style="display:flex;align-items:center;justify-content:center;min-height:200px;color:var(--sre-text-tertiary);">
      {{ t('explore.selectDatasource') }}
    </div>

    <!-- query bar -->
    <div v-if="selectedDsId" style="display:flex;align-items:flex-start;gap:8px;padding:12px 16px;background:var(--sre-bg-card);border-radius:8px;border:1px solid var(--sre-border);margin-bottom:12px;">
      <textarea
        v-model="expression"
        :placeholder="isLogs ? t('explore.logQueryPlaceholder') : t('explore.promqlPlaceholder')"
        style="flex:1;padding:6px 10px;border-radius:4px;border:1px solid var(--sre-border);background:var(--sre-bg-sunken);color:var(--sre-text-primary);font-size:13px;font-family:monospace;resize:vertical;min-height:32px;"
        rows="1"
        @keyup="onKey"
      ></textarea>
      <input v-if="isLogs" v-model.number="logLimit" type="number" min="10" max="10000" style="width:110px;padding:6px 10px;border-radius:4px;border:1px solid var(--sre-border);background:var(--sre-bg-sunken);color:var(--sre-text-primary);font-size:13px;" :placeholder="t('explore.limit')" />
      <button :disabled="loading || !expression.trim()" @click="run" style="padding:6px 16px;border-radius:6px;border:none;background:var(--sre-primary,#18a058);color:white;font-size:13px;cursor:pointer;white-space:nowrap;">
        {{ loading ? '...' : t('explore.runQuery') }}
      </button>
      <span style="font-size:11px;color:var(--sre-text-tertiary);align-self:center;white-space:nowrap;">Ctrl+Enter</span>
    </div>

    <!-- error -->
    <div v-if="errorMsg" style="padding:10px 14px;border-radius:6px;background:rgba(208,48,80,0.12);color:#d03050;font-size:13px;margin-bottom:12px;">{{ errorMsg }}</div>

    <!-- metric results -->
    <div v-if="metricRows.length && !isLogs" style="background:var(--sre-bg-card);border-radius:12px;padding:16px;">
      <div style="font-size:13px;color:var(--sre-text-secondary);margin-bottom:12px;">{{ t('explore.table') }} — {{ metricRows.length }} rows</div>
      <table style="width:100%;border-collapse:collapse;font-size:13px;">
        <thead><tr>
          <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--sre-border);color:var(--sre-text-secondary);">{{ t('explore.metricName') || 'Metric' }}</th>
          <th style="text-align:right;padding:6px 8px;border-bottom:1px solid var(--sre-border);color:var(--sre-text-secondary);width:140px;">{{ t('explore.value') || 'Value' }}</th>
          <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--sre-border);color:var(--sre-text-secondary);">{{ t('explore.labelsHeader') || 'Labels' }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="r in metricRows" :key="r._key" style="border-bottom:1px solid rgba(128,128,128,0.06);">
            <td style="padding:6px 8px;color:var(--sre-text-primary);">{{ r.name }}</td>
            <td style="padding:6px 8px;text-align:right;font-family:monospace;color:var(--sre-text-primary);">{{ r.value }}</td>
            <td style="padding:6px 8px;color:var(--sre-text-secondary);font-size:11px;">{{ formatLabels(r.labels) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- log results -->
    <div v-if="isLogs && logEntries.length" style="background:var(--sre-bg-card);border-radius:12px;padding:16px;">
      <div style="font-size:13px;color:var(--sre-text-secondary);margin-bottom:12px;">
        {{ t('explore.showing') }} {{ logEntries.length }}
        <template v-if="logTotal > 0"> / {{ logTotal }}</template>
        {{ t('explore.entries') }}
        <span v-if="logTruncated" style="margin-left:8px;padding:2px 6px;border-radius:4px;background:rgba(240,160,32,0.15);color:#f0a020;font-size:11px;">{{ t('explore.truncated') }}</span>
      </div>
      <table style="width:100%;border-collapse:collapse;font-size:13px;">
        <thead><tr>
          <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--sre-border);color:var(--sre-text-secondary);width:200px;">{{ t('explore.logTime') || 'Time' }}</th>
          <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--sre-border);color:var(--sre-text-secondary);">{{ t('explore.logMessage') || 'Message' }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="e in logEntries" :key="e._key" style="border-bottom:1px solid rgba(128,128,128,0.06);">
            <td style="padding:6px 8px;color:var(--sre-text-secondary);white-space:nowrap;font-size:11px;">{{ fmtTs(e.timestamp) }}</td>
            <td style="padding:6px 8px;color:var(--sre-text-primary);word-break:break-all;">{{ e.message || '-' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- empty -->
    <div v-if="!loading && !errorMsg && expression.trim() && !metricRows.length && !logEntries.length" style="display:flex;align-items:center;justify-content:center;min-height:200px;color:var(--sre-text-tertiary);">
      {{ t('explore.logEmptyDesc') || 'No results' }}
    </div>
  </div>
</template>

<script lang="ts">
// helper functions (not reactive, outside setup)
function formatLabels(lbs: any): string {
  if (!lbs) return '-'
  const parts: string[] = []
  for (const k of Object.keys(lbs)) {
    if (k !== '__name__') parts.push(`${k}=${lbs[k]}`)
  }
  return parts.length ? parts.join(' ') : '-'
}
function fmtTs(ts: any): string {
  if (!ts) return '-'
  try { return new Date(ts).toLocaleString() } catch { return String(ts) }
}
</script>
