<script setup lang="ts">
import { ref, computed, onMounted, watch, h, shallowRef } from 'vue'
import { NDataTable, NEmpty, NSpin as NSpinComponent } from 'naive-ui'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, BarChart } from 'echarts/charts'
import { TooltipComponent, LegendComponent, GridComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import { datasourceApi } from '@/api'
import type { PanelConfig, PanelTarget } from '@/types/dashboard'
import type { QueryResponse } from '@/types'

use([CanvasRenderer, LineChart, BarChart, TooltipComponent, LegendComponent, GridComponent])

const props = defineProps<{
  panel: PanelConfig
  timeRange: { start: number; end: number }
}>()

const loading = ref(false)
const error = ref('')
const series = ref<QueryResponse['series']>([])
const resultType = ref<'vector' | 'matrix' | 'logs' | null>(null)

const stepAuto = computed(() => {
  const diff = (props.timeRange.end - props.timeRange.start) / 1000
  if (diff <= 3600) return '15s'
  if (diff <= 21600) return '1m'
  if (diff <= 86400) return '5m'
  return '15m'
})

async function fetchData() {
  const targets = props.panel.targets
  if (!targets?.length) return

  loading.value = true
  error.value = ''
  series.value = []

  try {
    const allSeries: QueryResponse['series'] = []
    let type: 'vector' | 'matrix' | 'logs' | null = null

    for (const t of targets) {
      if (!t.datasourceId || !t.expression?.trim()) continue
      const res = await datasourceApi.rangeQuery(t.datasourceId, {
        expression: t.expression,
        start: Math.floor(props.timeRange.start / 1000),
        end: Math.floor(props.timeRange.end / 1000),
        step: stepAuto.value,
      })
      const data = res.data.data
      if (data.result_type) type = data.result_type
      if (data.series) {
        for (const s of data.series) {
          const labelStr = Object.entries(s.labels || {})
            .filter(([k]) => k !== '__name__')
            .map(([k, v]) => `${k}=${v}`)
            .join(',')
          const name = t.legendFormat
            ? t.legendFormat.replace(/\{\{\.label\}\}/g, labelStr)
            : (labelStr || s.labels?.__name__ || 'value')
          allSeries.push({ ...s, labels: { ...s.labels, __panel_name: name } })
        }
      }
    }
    resultType.value = type
    series.value = allSeries
  } catch (err: any) {
    error.value = err?.response?.data?.message || err?.message || 'Query failed'
  } finally {
    loading.value = false
  }
}

const statValue = computed(() => {
  if (!series.value.length) return null
  const s = series.value[0]
  if (s.values?.length) return s.values[s.values.length - 1].value
  return null
})

const statColor = computed(() => {
  const val = statValue.value
  const thresholds: { value: number; color: string }[] | undefined = props.panel.options?.thresholds
  if (val == null || !thresholds?.length) {
    return props.panel.options?.color || 'var(--sre-text-primary)'
  }
  const sorted = [...thresholds].sort((a, b) => a.value - b.value)
  let color = props.panel.options?.color || sorted[0]?.color || 'var(--sre-text-primary)'
  for (const t of sorted) {
    if (val >= t.value) color = t.color
  }
  return color
})

const statSeriesName = computed(() => {
  if (!series.value.length) return ''
  return series.value[0].labels?.__panel_name || series.value[0].labels?.__name__ || 'value'
})

const chartOption = computed(() => {
  if (!series.value.length) return null

  const xData: string[] = []
  const seriesList: any[] = []
  const seen = new Map<string, boolean>()

  for (const s of series.value) {
    const name = s.labels?.__panel_name || s.labels?.__name__ || 'value'
    if (!seen.has(name)) {
      seen.set(name, true)
      seriesList.push({ name, type: 'line', smooth: true, symbol: 'none', data: [] as [string, number][] })
    }
    const target = seriesList.find(sl => sl.name === name)
    if (target) {
      for (const v of s.values) {
        const ts = new Date(v.ts * 1000).toLocaleTimeString()
        target.data.push([ts, v.value])
      }
    }
  }

  if (resultType.value === 'matrix') {
    const allTimes = new Set<string>()
    for (const sl of seriesList) {
      for (const d of sl.data) allTimes.add(d[0])
    }
    const sorted = Array.from(allTimes).sort()
    for (const sl of seriesList) {
      const timeMap = new Map(sl.data.map((d: [string, number]) => [d[0], d[1]]))
      sl.data = sorted.map(t => timeMap.get(t) ?? null)
    }
    return {
      tooltip: { trigger: 'axis' as const },
      legend: { type: 'scroll' as const, bottom: 0, textStyle: { fontSize: 11 } },
      grid: { left: 50, right: 16, top: 12, bottom: 40 },
      xAxis: { type: 'category' as const, data: sorted },
      yAxis: { type: 'value' as const },
      series: seriesList,
    }
  }

  return {
    tooltip: { trigger: 'axis' as const },
    legend: { show: false },
    grid: { left: 50, right: 16, top: 12, bottom: 30 },
    xAxis: { type: 'category' as const, data: xData },
    yAxis: { type: 'value' as const },
    series: seriesList,
  }
})

const tableData = computed(() => {
  const rows: { labels: Record<string, string>; value: number; _key: number }[] = []
  let idx = 0
  for (const s of series.value) {
    for (const v of s.values) {
      rows.push({ labels: s.labels, value: v.value, _key: idx++ })
    }
  }
  return rows
})

const tableColumns = computed(() => {
  const keys = new Set<string>()
  for (const s of series.value) {
    Object.keys(s.labels || {}).forEach(k => { if (k !== '__panel_name') keys.add(k) })
  }
  const cols: any[] = Array.from(keys).map(k => ({
    title: k,
    key: k,
    ellipsis: { tooltip: true },
    render(row: any) { return row.labels?.[k] || '-' },
  }))
  cols.push({ title: 'Value', key: 'value', width: 120, render(row: any) { return row.value?.toFixed(4) || '-' } })
  return cols
})

let timeout: ReturnType<typeof setTimeout>
watch(() => [props.timeRange, props.panel.targets], () => {
  clearTimeout(timeout)
  timeout = setTimeout(fetchData, 100)
}, { deep: true })

onMounted(fetchData)
</script>

<template>
  <div class="panel-card">
    <div class="panel-card-header">
      <span class="panel-title">{{ panel.title || 'Panel' }}</span>
      <NSpinComponent v-if="loading" :size="14" />
      <span v-if="error" class="panel-error">{{ error }}</span>
    </div>
    <div class="panel-card-body">
      <template v-if="loading && !series.length">
        <div class="panel-loading"><NSpinComponent :size="24" /></div>
      </template>
      <template v-else-if="error && !series.length">
        <NEmpty :description="error" size="small" />
      </template>
      <template v-else-if="!series.length">
        <NEmpty description="No data" size="small" />
      </template>

      <!-- Timeseries -->
      <template v-else-if="panel.type === 'timeseries' || !panel.type">
        <VChart v-if="chartOption" :option="chartOption" autoresize style="height: 100%" />
      </template>

      <!-- Stat -->
      <template v-else-if="panel.type === 'stat'">
        <div class="stat-display" :style="{ color: statColor }">
          <div class="stat-value">{{ statValue?.toFixed(2) ?? '-' }}</div>
          <div class="stat-label">{{ statSeriesName }}</div>
        </div>
      </template>

      <!-- Table -->
      <template v-else-if="panel.type === 'table'">
        <NDataTable
          :columns="tableColumns"
          :data="tableData"
          :max-height="280"
          :row-key="(row: any) => row._key"
          size="small"
          striped
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.panel-card {
  background: var(--sre-bg-card);
  border: 1px solid var(--sre-border);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}
.panel-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--sre-border);
}
.panel-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--sre-text-secondary);
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.panel-error {
  font-size: 11px;
  color: var(--sre-danger);
}
.panel-card-body {
  flex: 1;
  min-height: 0;
  padding: 8px;
}
.panel-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 120px;
}
.stat-display {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 100px;
}
.stat-value {
  font-size: 36px;
  font-weight: 700;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
}
.stat-label {
  font-size: 12px;
  color: var(--sre-text-tertiary);
  margin-top: 4px;
}
</style>
