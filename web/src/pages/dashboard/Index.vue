<script setup lang="ts">
import { h, ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NTag } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { PieChart, GaugeChart, LineChart, BarChart } from 'echarts/charts'
import {
  TooltipComponent,
  LegendComponent,
  GridComponent,
} from 'echarts/components'
import VChart from 'vue-echarts'
import { dashboardApi, alertEventApi, engineApi } from '@/api'
import type { DashboardStats, MTTRStats, MTTRTrendPoint, AlertEvent, EngineStatus, AlertTrendPoint, TopRuleItem } from '@/types'
import { formatTime } from '@/utils/format'
import { getSeverityType, getEventStatusType } from '@/utils/alert'
import PageHeader from '@/components/common/PageHeader.vue'
import GlowCard from '@/components/common/GlowCard.vue'
import AnimatedNumber from '@/components/common/AnimatedNumber.vue'
import {
  AlertCircleOutline,
  ServerOutline,
  CheckmarkCircleOutline,
  ReaderOutline,
  PulseOutline,
  TimeOutline,
  PeopleOutline,
  LayersOutline,
  DownloadOutline,
} from '@vicons/ionicons5'

use([CanvasRenderer, PieChart, GaugeChart, LineChart, BarChart, TooltipComponent, LegendComponent, GridComponent])

const router = useRouter()
const message = useMessage()
const { t } = useI18n()
const loading = ref(false)
const eventsLoading = ref(false)
const mttrLoading = ref(false)

const stats = ref<DashboardStats>({
  total_datasources: 0,
  total_rules: 0,
  active_alerts: 0,
  resolved_today: 0,
  total_users: 0,
  total_teams: 0,
  severity_breakdown: { critical: 0, warning: 0, info: 0 },
})

const engineStatus = ref<EngineStatus>({
  running: false,
  total_rules: 0,
  active_alerts: 0,
  uptime: '',
})

const recentAlerts = ref<AlertEvent[]>([])

const mttrWindowOptions = [
  { label: '1h', value: 1 },
  { label: '6h', value: 6 },
  { label: '24h', value: 24 },
  { label: '7d', value: 168 },
  { label: '30d', value: 720 },
]
const mttrWindow = ref(24)

const emptyMetric = { mean: -1, p50: -1, p95: -1, count: 0 }
const mttrStats = ref<MTTRStats>({
  window_hours: 24,
  mtta: { ...emptyMetric },
  mttr: { ...emptyMetric },
  by_severity: [
    { severity: 'critical', mtta: { ...emptyMetric }, mttr: { ...emptyMetric } },
    { severity: 'warning',  mtta: { ...emptyMetric }, mttr: { ...emptyMetric } },
    { severity: 'info',     mtta: { ...emptyMetric }, mttr: { ...emptyMetric } },
  ],
  // legacy mirrors — still populated by the server, referenced by older code paths
  mtta_seconds: -1,
  mttr_seconds: -1,
  acked_count: 0,
  resolved_count: 0,
})

const mttrTrend = ref<MTTRTrendPoint[]>([])
const mttrTrendLoading = ref(false)
const mttrTrendDays = ref(30)
const mttrTrendDayOptions = [
  { label: () => t('dashboard.last7d'),  value: 7 },
  { label: () => t('dashboard.last14d'), value: 14 },
  { label: () => t('dashboard.last30d'), value: 30 },
  { label: () => t('dashboard.last90d'), value: 90 },
]

const trendData = ref<AlertTrendPoint[]>([])
const topRules = ref<TopRuleItem[]>([])
const trendDays = ref(30)
const trendLoading = ref(false)

const trendDayOptions = [
  { label: () => t('dashboard.last7d'), value: 7 },
  { label: () => t('dashboard.last14d'), value: 14 },
  { label: () => t('dashboard.last30d'), value: 30 },
  { label: () => t('dashboard.last90d'), value: 90 },
]

const statCards = [
  { titleKey: 'dashboard.activeAlerts', key: 'active_alerts' as const, icon: AlertCircleOutline, color: '#e88080', gradient: 'linear-gradient(135deg, #e88080, #c0392b)' },
  { titleKey: 'dashboard.dataSources', key: 'total_datasources' as const, icon: ServerOutline, color: '#18a058', gradient: 'linear-gradient(135deg, #18a058, #0d6e3e)' },
  { titleKey: 'dashboard.resolvedToday', key: 'resolved_today' as const, icon: CheckmarkCircleOutline, color: '#70c0e8', gradient: 'linear-gradient(135deg, #70c0e8, #3498db)' },
  { titleKey: 'dashboard.totalRules', key: 'total_rules' as const, icon: ReaderOutline, color: '#f2c97d', gradient: 'linear-gradient(135deg, #f2c97d, #e67e22)' },
]

// ECharts: severity donut
const severityChartOption = computed(() => ({
  backgroundColor: 'transparent',
  tooltip: {
    trigger: 'item',
    formatter: '{b}: {c} ({d}%)',
    backgroundColor: 'rgba(0,0,0,0.75)',
    borderColor: 'transparent',
    textStyle: { color: '#fff', fontSize: 12 },
  },
  legend: {
    orient: 'vertical',
    right: '5%',
    top: 'center',
    textStyle: { color: '#aaa', fontSize: 12 },
    itemWidth: 10,
    itemHeight: 10,
  },
  series: [{
    name: 'Severity',
    type: 'pie',
    radius: ['52%', '75%'],
    center: ['38%', '50%'],
    avoidLabelOverlap: false,
    itemStyle: { borderRadius: 6, borderColor: 'transparent', borderWidth: 2 },
    label: { show: false },
    emphasis: {
      label: { show: true, fontSize: 14, fontWeight: 'bold', color: '#fff' },
      itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.4)' },
    },
    data: [
      { value: stats.value.severity_breakdown?.critical ?? 0, name: t('alert.critical'), itemStyle: { color: '#e88080' } },
      { value: stats.value.severity_breakdown?.warning ?? 0, name: t('alert.warning'), itemStyle: { color: '#f2c97d' } },
      { value: stats.value.severity_breakdown?.info ?? 0, name: t('alert.info'), itemStyle: { color: '#70c0e8' } },
    ],
  }],
}))

// Duration formatter shortcut — `-1` means "no data" and renders as a dash.
function fmtDuration(seconds: number): string {
  return seconds < 0 ? '—' : formatDuration(seconds)
}

function sevLabel(sev: string): string {
  const key = `alert.${sev}`
  return t(key) || sev
}

// ECharts: MTTR trend (daily means for MTTA + MTTR)
const mttrTrendChartOption = computed(() => {
  const dates = mttrTrend.value.map(p => p.date)
  // ECharts draws gaps automatically when we use `null` instead of a negative sentinel.
  const asMinutes = (s: number) => (s < 0 ? null : +(s / 60).toFixed(1))
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(0,0,0,0.75)',
      borderColor: 'transparent',
      textStyle: { color: '#fff', fontSize: 12 },
      valueFormatter: (v: number | null) => v == null ? '—' : `${v} min`,
    },
    legend: {
      data: ['MTTA', 'MTTR'],
      textStyle: { color: '#888', fontSize: 11 },
      itemWidth: 12,
      itemHeight: 2,
      right: 0,
      top: 0,
    },
    grid: { left: '2%', right: '2%', bottom: '6%', top: '18%', containLabel: true },
    xAxis: {
      type: 'category',
      data: dates,
      axisLabel: { color: '#888', fontSize: 10 },
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.08)' } },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      name: 'min',
      nameTextStyle: { color: '#666', fontSize: 10 },
      axisLabel: { color: '#888', fontSize: 10 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
    },
    series: [
      {
        name: 'MTTA',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        data: mttrTrend.value.map(p => asMinutes(p.mtta_seconds)),
        connectNulls: false,
        lineStyle: { color: '#f59e0b', width: 2 },
        itemStyle: { color: '#f59e0b' },
      },
      {
        name: 'MTTR',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        data: mttrTrend.value.map(p => asMinutes(p.mttr_seconds)),
        connectNulls: false,
        lineStyle: { color: '#18a058', width: 2 },
        itemStyle: { color: '#18a058' },
        areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [
          { offset: 0, color: 'rgba(24,160,88,0.18)' }, { offset: 1, color: 'rgba(24,160,88,0.01)' }
        ] } },
      },
    ],
  }
})

const alertColumns = [
  {
    title: () => t('alert.severity'),
    key: 'severity',
    width: 100,
    render: (row: AlertEvent) =>
      h(NTag, { type: getSeverityType(row.severity), size: 'small', round: true }, { default: () => row.severity.toUpperCase() }),
  },
  {
    title: () => t('alert.alertName'),
    key: 'alert_name',
    ellipsis: { tooltip: true },
    render: (row: AlertEvent) =>
      h('a', {
        class: 'alert-link',
        onClick: () => router.push(`/alerts/events/${row.id}`),
      }, row.alert_name),
  },
  {
    title: () => t('common.status'),
    key: 'status',
    width: 120,
    render: (row: AlertEvent) =>
      h(NTag, { type: getEventStatusType(row.status), size: 'small' }, { default: () => row.status }),
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
    width: 180,
    render: (row: AlertEvent) => h('span', { style: 'font-size:12px' }, formatTime(row.fired_at)),
  },
  {
    title: () => t('alert.fireCount'),
    key: 'fire_count',
    width: 70,
    align: 'center' as const,
  },
]

async function fetchStats() {
  loading.value = true
  try {
    const { data } = await dashboardApi.getStats()
    stats.value = data.data
  } catch (err: any) {
    message.error(err.message || t('dashboard.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function fetchEngineStatus() {
  try {
    const { data } = await engineApi.getStatus()
    engineStatus.value = data.data
  } catch {
    engineStatus.value = { running: false, total_rules: 0, active_alerts: 0, uptime: '' }
  }
}

function formatDuration(seconds: number): string {
  if (seconds < 0) return '-'
  if (seconds < 60) return `${Math.round(seconds)}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  if (seconds < 86400) return `${(seconds / 3600).toFixed(1)}h`
  return `${(seconds / 86400).toFixed(1)}d`
}

async function fetchMTTRStats() {
  mttrLoading.value = true
  try {
    const { data } = await dashboardApi.getMTTRStats(mttrWindow.value)
    mttrStats.value = data.data
  } catch {
    // non-critical
  } finally {
    mttrLoading.value = false
  }
}

async function fetchMTTRTrend() {
  mttrTrendLoading.value = true
  try {
    const { data } = await dashboardApi.getMTTRTrend(mttrTrendDays.value)
    mttrTrend.value = data.data || []
  } catch {
    // non-critical
  } finally {
    mttrTrendLoading.value = false
  }
}

const trendChartOption = computed(() => ({
  backgroundColor: 'transparent',
  tooltip: { trigger: 'axis' },
  legend: { data: [t('dashboard.fired'), t('dashboard.resolved')], textStyle: { color: '#aaa' } },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: { type: 'category', data: trendData.value.map(d => d.date), axisLabel: { color: '#888' }, axisLine: { lineStyle: { color: '#333' } } },
  yAxis: { type: 'value', axisLabel: { color: '#888' }, splitLine: { lineStyle: { color: 'rgba(255,255,255,0.06)' } } },
  series: [
    { name: t('dashboard.fired'), type: 'line', smooth: true, data: trendData.value.map(d => d.fired_count),
      lineStyle: { color: '#e88080', width: 2 }, itemStyle: { color: '#e88080' },
      areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(232,128,128,0.3)' }, { offset: 1, color: 'rgba(232,128,128,0.02)' }] } } },
    { name: t('dashboard.resolved'), type: 'line', smooth: true, data: trendData.value.map(d => d.resolved_count),
      lineStyle: { color: '#18a058', width: 2 }, itemStyle: { color: '#18a058' },
      areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(24,160,88,0.3)' }, { offset: 1, color: 'rgba(24,160,88,0.02)' }] } } },
  ],
}))

const topRulesChartOption = computed(() => ({
  backgroundColor: 'transparent',
  tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
  grid: { left: '3%', right: '10%', bottom: '3%', top: '3%', containLabel: true },
  xAxis: { type: 'value', axisLabel: { color: '#888' }, splitLine: { lineStyle: { color: 'rgba(255,255,255,0.06)' } } },
  yAxis: { type: 'category', data: topRules.value.map(r => r.alert_name).reverse(), axisLabel: { color: '#aaa', fontSize: 11, width: 120, overflow: 'truncate' } },
  series: [{
    type: 'bar', data: topRules.value.map(r => r.count).reverse(),
    itemStyle: { color: { type: 'linear', x: 0, y: 0, x2: 1, y2: 0, colorStops: [{ offset: 0, color: '#e8808033' }, { offset: 1, color: '#e88080' }] }, borderRadius: [0, 4, 4, 0] },
    barMaxWidth: 20,
  }],
}))

async function fetchTrendData() {
  trendLoading.value = true
  try {
    const [trendRes, topRes] = await Promise.all([
      dashboardApi.getAlertTrend(trendDays.value),
      dashboardApi.getTopRules(trendDays.value, 10),
    ])
    trendData.value = trendRes.data.data || []
    topRules.value = topRes.data.data || []
  } catch {
    // non-critical
  } finally {
    trendLoading.value = false
  }
}

async function fetchRecentAlerts() {
  eventsLoading.value = true
  try {
    const { data } = await alertEventApi.list({ page: 1, page_size: 10, status: ['firing'] })
    recentAlerts.value = data.data.list || []
  } catch (err: any) {
    message.error(err.message || t('dashboard.loadAlertsFailed'))
  } finally {
    eventsLoading.value = false
  }
}

// ===== Report Export =====
const showExportModal = ref(false)
// date picker uses [startTs, endTs] in ms; default last 30 days
const exportRange = ref<[number, number]>([
  Date.now() - 30 * 86400_000,
  Date.now(),
])

function handleExportReport() {
  if (!exportRange.value) return
  const fmt = (ts: number) => new Date(ts).toISOString().slice(0, 10)
  const url = dashboardApi.exportReportURL(fmt(exportRange.value[0]), fmt(exportRange.value[1]))
  const a = document.createElement('a')
  a.href = url
  a.download = `alert-report-${fmt(exportRange.value[0])}-to-${fmt(exportRange.value[1])}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  showExportModal.value = false
}

onMounted(() => {
  fetchStats()
  fetchEngineStatus()
  fetchRecentAlerts()
  fetchMTTRStats()
  fetchMTTRTrend()
  fetchTrendData()
})
</script>

<template>
  <div class="dashboard">
    <PageHeader :title="t('dashboard.title')" :subtitle="t('dashboard.subtitle')">
      <template #actions>
        <n-button size="small" @click="showExportModal = true">
          <template #icon><n-icon :component="DownloadOutline" /></template>
          {{ t('dashboard.exportReport') }}
        </n-button>
      </template>
    </PageHeader>

    <!-- Export modal -->
    <n-modal v-model:show="showExportModal" preset="card" :title="t('dashboard.exportReport')" style="max-width:420px">
      <n-form label-placement="top" size="small">
        <n-form-item :label="t('dashboard.exportDateRange')">
          <n-date-picker
            v-model:value="exportRange"
            type="daterange"
            clearable
            style="width:100%"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showExportModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :disabled="!exportRange" @click="handleExportReport">
            <template #icon><n-icon :component="DownloadOutline" /></template>
            {{ t('dashboard.downloadCSV') }}
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- ===== Bento Grid ===== -->
    <div class="bento-grid stagger-grid">

      <!-- A: Hero — Active Alerts (large, prominent) -->
      <GlowCard
        variant="critical"
        :glow="stats.active_alerts > 0"
        :conic="stats.active_alerts > 0 ? 'critical' : false"
        :tilt="true"
        padding="0"
        class="bento-hero"
      >
        <div class="hero-accent" />
        <div class="hero-body">
          <div class="hero-left">
            <div class="eyebrow" style="color: var(--sre-critical); margin-bottom: 8px">
              {{ t('dashboard.activeAlerts') }}
            </div>
            <div class="hero-number">
              <AnimatedNumber :value="stats.active_alerts" :duration="1200" class="hero-count" />
            </div>
            <div class="hero-sev-bars">
              <div
                v-for="sev in (['critical','warning','info'] as const)"
                :key="sev"
                class="hero-sev-bar"
                :class="`hero-sev-bar--${sev}`"
              >
                <span class="hero-sev-label">{{ t(`alert.${sev}`) }}</span>
                <div class="hero-sev-track">
                  <div
                    class="hero-sev-fill"
                    :style="{
                      width: stats.active_alerts
                        ? ((stats.severity_breakdown?.[sev] ?? 0) / stats.active_alerts * 100) + '%'
                        : '0%'
                    }"
                  />
                </div>
                <span class="hero-sev-count number-display">{{ stats.severity_breakdown?.[sev] ?? 0 }}</span>
              </div>
            </div>
          </div>
          <div class="hero-right">
            <n-icon :component="AlertCircleOutline" :size="56" style="opacity:0.12; color: var(--sre-critical)" />
          </div>
        </div>
      </GlowCard>

      <!-- B: MTTA/MTTR (tall right column) -->
      <GlowCard variant="default" :tilt="true" padding="0" class="bento-mtt">
        <div class="panel-card__header mttr-card__header" style="padding: 16px 20px 12px">
          <div class="mttr-card__title-row">
            <n-icon :component="TimeOutline" size="14" />
            <span class="panel-card__title">MTTA / MTTR</span>
          </div>
          <n-radio-group v-model:value="mttrWindow" size="small" @update:value="fetchMTTRStats">
            <n-radio-button v-for="opt in mttrWindowOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</n-radio-button>
          </n-radio-group>
        </div>
        <n-spin :show="mttrLoading">
          <div style="padding: 0 20px 16px">
            <div class="mttr-hero">
              <div class="mttr-metric mttr-metric--ack">
                <div class="mttr-metric__eyebrow">MTTA · P50</div>
                <div class="mttr-metric__value number-display">{{ fmtDuration(mttrStats.mtta.p50) }}</div>
                <div class="mttr-metric__subs">
                  <span class="mttr-metric__sub"><em>mean</em> {{ fmtDuration(mttrStats.mtta.mean) }}</span>
                  <span class="mttr-metric__dot">·</span>
                  <span class="mttr-metric__sub"><em>P95</em> {{ fmtDuration(mttrStats.mtta.p95) }}</span>
                </div>
              </div>
              <div class="mttr-hero__divider" />
              <div class="mttr-metric mttr-metric--resolve">
                <div class="mttr-metric__eyebrow">MTTR · P50</div>
                <div class="mttr-metric__value number-display">{{ fmtDuration(mttrStats.mttr.p50) }}</div>
                <div class="mttr-metric__subs">
                  <span class="mttr-metric__sub"><em>mean</em> {{ fmtDuration(mttrStats.mttr.mean) }}</span>
                  <span class="mttr-metric__dot">·</span>
                  <span class="mttr-metric__sub"><em>P95</em> {{ fmtDuration(mttrStats.mttr.p95) }}</span>
                </div>
              </div>
            </div>
            <div class="mttr-sev" style="margin-top:12px">
              <div class="mttr-sev__head eyebrow">{{ t('dashboard.bySeverity') }}</div>
              <div v-for="sev in mttrStats.by_severity" :key="sev.severity" class="mttr-sev__row" :class="`mttr-sev__row--${sev.severity}`">
                <span class="mttr-sev__tag">{{ sevLabel(sev.severity) }}</span>
                <div class="mttr-sev__metric"><span class="mttr-sev__label">MTTA</span><span class="mttr-sev__val number-display">{{ fmtDuration(sev.mtta.mean) }}</span></div>
                <div class="mttr-sev__metric"><span class="mttr-sev__label">MTTR</span><span class="mttr-sev__val number-display">{{ fmtDuration(sev.mttr.mean) }}</span></div>
              </div>
            </div>
          </div>
        </n-spin>
      </GlowCard>

      <!-- C-F: Small stat cards -->
      <GlowCard v-for="(card, idx) in statCards" :key="card.key"
        variant="default" :tilt="true" padding="0"
        :style="{ '--sre-stagger-i': idx + 2 }"
        class="bento-stat stagger-item"
      >
        <div class="stat-card__accent" :style="{ background: card.gradient }" />
        <div class="stat-card__body">
          <div class="stat-card__icon" :style="{ background: card.color + '18', color: card.color }">
            <n-icon :component="card.icon" :size="20" />
          </div>
          <div class="stat-card__info">
            <div class="stat-card__label">{{ t(card.titleKey) }}</div>
            <AnimatedNumber :value="stats[card.key]" class="stat-card__value" style="font-size:22px" />
          </div>
        </div>
      </GlowCard>

      <!-- G: Engine status -->
      <GlowCard
        :variant="engineStatus.running ? 'success' : 'critical'"
        :glow="!engineStatus.running"
        :tilt="true"
        padding="0"
        :style="{ '--sre-stagger-i': statCards.length + 2 }"
        class="bento-stat stagger-item"
      >
        <div class="stat-card__accent" :style="{ background: engineStatus.running ? 'linear-gradient(135deg,#18a058,#0d6e3e)' : 'linear-gradient(135deg,#e88080,#c0392b)' }" />
        <div class="stat-card__body">
          <div class="stat-card__icon" :style="{ background: (engineStatus.running ? '#18a058' : '#e88080') + '18', color: engineStatus.running ? '#18a058' : '#e88080' }">
            <n-icon :component="PulseOutline" :size="20" />
          </div>
          <div class="stat-card__info">
            <div class="stat-card__label">{{ t('engine.title') }}</div>
            <div class="engine-status-row">
              <span class="engine-dot" :class="engineStatus.running ? 'engine-dot--running' : 'engine-dot--stopped'" />
              <span class="stat-card__value" style="font-size:16px">{{ engineStatus.running ? t('engine.running') : t('engine.stopped') }}</span>
            </div>
          </div>
        </div>
      </GlowCard>

      <!-- H: Severity Donut -->
      <div class="panel-card bento-donut">
        <div class="panel-card__header">
          <span class="panel-card__title">{{ t('dashboard.severityDistribution') }}</span>
        </div>
        <v-chart :option="severityChartOption" autoresize style="height:160px" />
        <div class="sev-legend">
          <div v-for="s in (['critical','warning','info'] as const)" :key="s" class="sev-item">
            <span class="sev-dot" :class="`sev-dot--${s}`" />
            <span class="sev-label">{{ t(`alert.${s}`) }}</span>
            <span class="sev-count">{{ stats.severity_breakdown?.[s] ?? 0 }}</span>
          </div>
        </div>
      </div>

      <!-- I: MTTR Trend -->
      <div class="panel-card bento-trend">
        <div class="panel-card__header" style="justify-content:space-between">
          <div style="display:flex;align-items:center;gap:6px">
            <n-icon :component="TimeOutline" size="14" />
            <span class="panel-card__title">{{ t('dashboard.mttrTrend') }}</span>
          </div>
          <n-radio-group v-model:value="mttrTrendDays" size="small" @update:value="fetchMTTRTrend">
            <n-radio-button v-for="opt in mttrTrendDayOptions" :key="opt.value" :value="opt.value">{{ opt.label() }}</n-radio-button>
          </n-radio-group>
        </div>
        <n-spin :show="mttrTrendLoading">
          <v-chart :option="mttrTrendChartOption" autoresize style="height:220px" />
        </n-spin>
      </div>

      <!-- J: Alert Trend + Top Rules row -->
      <div class="panel-card bento-alert-trend">
        <div class="panel-card__header" style="justify-content:space-between">
          <span class="panel-card__title">{{ t('dashboard.alertTrend') }}</span>
          <n-radio-group v-model:value="trendDays" size="small" @update:value="fetchTrendData">
            <n-radio-button v-for="opt in trendDayOptions" :key="opt.value" :value="opt.value">{{ opt.label() }}</n-radio-button>
          </n-radio-group>
        </div>
        <n-spin :show="trendLoading">
          <v-chart :option="trendChartOption" autoresize style="height:250px" />
        </n-spin>
      </div>

      <div class="panel-card bento-top-rules">
        <div class="panel-card__header">
          <span class="panel-card__title">{{ t('dashboard.topRules') }}</span>
        </div>
        <n-spin :show="trendLoading">
          <v-chart :option="topRulesChartOption" autoresize style="height:250px" />
        </n-spin>
      </div>

    </div>

    <!-- Recent Alerts Table -->
    <div class="panel-card">
      <div class="panel-card__header" style="justify-content:space-between">
        <span class="panel-card__title">{{ t('dashboard.recentAlerts') }}</span>
        <n-button text type="primary" size="small" @click="router.push('/alerts/events')">
          {{ t('dashboard.viewAll') }} →
        </n-button>
      </div>

      <n-data-table
        v-if="recentAlerts.length > 0 || eventsLoading"
        :loading="eventsLoading"
        :columns="alertColumns"
        :data="recentAlerts"
        :row-key="(row: AlertEvent) => row.id"
        :bordered="false"
        size="small"
        :pagination="false"
        :row-class-name="(row: AlertEvent) => row.severity === 'critical' ? 'row-critical' : row.severity === 'warning' ? 'row-warning' : ''"
      />

      <n-empty
        v-if="!eventsLoading && recentAlerts.length === 0"
        :description="t('dashboard.noAlerts')"
        style="padding: 40px 0"
      >
        <template #extra>
          <n-button type="primary" size="small" @click="router.push('/datasources')">
            {{ t('dashboard.configDatasources') }}
          </n-button>
        </template>
      </n-empty>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
  max-width: 1440px;
}

/* ===== Bento Grid ===== */
.bento-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: 1.15fr 1fr 1fr 1fr 0.9fr;
  grid-template-rows: auto auto auto auto;
  grid-template-areas:
    "hero hero mtt  mtt  mtt"
    "s1   s2   s3   s4   s5"
    "dnt  dnt  trnd trnd trnd"
    "atr  atr  atr  tpr  tpr";
  margin-bottom: 20px;
}

.bento-hero       { grid-area: hero; overflow: hidden; }
.bento-mtt        { grid-area: mtt;  overflow: hidden; }
.bento-stat       { overflow: hidden; }
.bento-stat:nth-child(3) { grid-area: s1; }
.bento-stat:nth-child(4) { grid-area: s2; }
.bento-stat:nth-child(5) { grid-area: s3; }
.bento-stat:nth-child(6) { grid-area: s4; }
.bento-stat:nth-child(7) { grid-area: s5; }
.bento-donut      { grid-area: dnt; }
.bento-trend      { grid-area: trnd; }
.bento-alert-trend { grid-area: atr; }
.bento-top-rules  { grid-area: tpr; }

/* Responsive: ≤ 1280px  */
@media (max-width: 1280px) {
  .bento-grid {
    grid-template-columns: 1fr 1fr 1fr;
    grid-template-areas:
      "hero hero mtt"
      "s1   s2   mtt"
      "s3   s4   s5"
      "dnt  trnd trnd"
      "atr  atr  tpr";
  }
}
@media (max-width: 768px) {
  .bento-grid {
    grid-template-columns: 1fr;
    grid-template-areas:
      "hero" "mtt" "s1" "s2" "s3" "s4" "s5"
      "dnt" "trnd" "atr" "tpr";
  }
}

/* Stagger entrance */
.stagger-item {
  animation: sre-slide-up var(--sre-duration-slow) var(--sre-ease-out) both;
  animation-delay: calc(var(--sre-stagger-i, 0) * 55ms);
}
.bento-hero  { animation: sre-slide-up 400ms var(--sre-ease-spring) 0ms both; }
.bento-mtt   { animation: sre-slide-up 400ms var(--sre-ease-spring) 60ms both; }
.bento-donut { animation: sre-slide-up 360ms var(--sre-ease-out) 220ms both; }
.bento-trend { animation: sre-slide-up 360ms var(--sre-ease-out) 280ms both; }

/* Hero card internals */
.hero-accent {
  height: 4px;
  background: linear-gradient(90deg, #ef4444, #f59e0b, #ef4444);
  background-size: 200% 100%;
  animation: sre-shimmer 3s linear infinite;
}
.hero-body {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 20px 20px;
  gap: 16px;
}
.hero-left { flex: 1; min-width: 0; }
.hero-right { flex-shrink: 0; align-self: center; }
.hero-count {
  font-size: clamp(48px, 6vw, 72px) !important;
  font-weight: var(--sre-fw-bold) !important;
  line-height: 1 !important;
  letter-spacing: -0.04em !important;
  color: var(--sre-text-primary);
  display: block;
  margin-bottom: 12px;
}
.hero-sev-bars { display: flex; flex-direction: column; gap: 6px; }
.hero-sev-bar  { display: flex; align-items: center; gap: 8px; }
.hero-sev-label { font-size: var(--sre-fs-xs); color: var(--sre-text-tertiary); width: 48px; flex-shrink: 0; text-transform: capitalize; }
.hero-sev-track { flex: 1; height: 4px; border-radius: 999px; background: var(--sre-bg-elevated); overflow: hidden; }
.hero-sev-fill  { height: 100%; border-radius: 999px; transition: width 1s var(--sre-ease-out); }
.hero-sev-bar--critical .hero-sev-fill { background: var(--sre-critical); }
.hero-sev-bar--warning  .hero-sev-fill { background: var(--sre-warning); }
.hero-sev-bar--info     .hero-sev-fill { background: var(--sre-info); }
.hero-sev-count { font-size: var(--sre-fs-xs); color: var(--sre-text-secondary); width: 24px; text-align: right; font-family: var(--sre-font-mono); }
.stat-card__accent {
  height: 3px;
  width: 100%;
}
.stat-card__body {
  display: flex;
  align-items: center;
  gap: var(--sre-space-4);
  padding: var(--sre-space-5) var(--sre-space-5);
}
.stat-card__icon {
  width: 48px;
  height: 48px;
  border-radius: var(--sre-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.04);
}
.stat-card__label {
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-tertiary);
  font-weight: var(--sre-fw-semibold);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  margin-bottom: var(--sre-space-1);
  white-space: nowrap;
}
.stat-card__value {
  font-size: var(--sre-fs-3xl);
  font-weight: var(--sre-fw-bold);
  color: var(--sre-text-primary);
  line-height: 1;
  font-family: var(--sre-font-mono);
  font-feature-settings: "tnum" 1, "lnum" 1;
  letter-spacing: -0.02em;
}
.engine-status-row {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
}
.engine-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.engine-dot--running {
  background: var(--sre-brand-500);
  box-shadow: 0 0 0 3px rgba(24,160,88,0.22),
              0 0 10px rgba(24,160,88,0.6);
  animation: engine-pulse 2.2s var(--sre-ease-in-out) infinite;
}
.engine-dot--stopped {
  background: var(--sre-critical);
  box-shadow: 0 0 0 3px rgba(239,68,68,0.18);
}
@keyframes engine-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(24,160,88,0.4), 0 0 8px rgba(24,160,88,0.6); }
  50%      { box-shadow: 0 0 0 8px rgba(24,160,88,0),   0 0 14px rgba(24,160,88,0.9); }
}
.engine-meta {
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-tertiary);
  margin-top: var(--sre-space-1);
}

/* ===== Panel Card (chart containers) ===== */
.panel-card {
  background: var(--sre-bg-card);
  border: 1px solid var(--sre-border);
  border-radius: var(--sre-radius-lg);
  padding: var(--sre-space-5) var(--sre-space-6);
  transition: border-color var(--sre-duration-base) var(--sre-ease-out),
              box-shadow var(--sre-duration-base) var(--sre-ease-out);
}
.panel-card:hover {
  border-color: var(--sre-border-strong);
}
.panel-card__header {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
  margin-bottom: var(--sre-space-4);
}
.panel-card__title {
  font-size: var(--sre-fs-xs);
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

/* ===== Severity legend ===== */
.sev-legend {
  display: flex;
  flex-direction: column;
  gap: var(--sre-space-2);
  margin-top: var(--sre-space-1);
}
.sev-item {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
  font-size: var(--sre-fs-sm);
  padding: 6px 8px;
  border-radius: var(--sre-radius-sm);
  transition: background var(--sre-duration-fast) var(--sre-ease-out);
}
.sev-item:hover { background: var(--sre-bg-hover); }
.sev-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.sev-dot--critical {
  background: var(--sre-critical);
  box-shadow: 0 0 6px rgba(239,68,68,0.55);
}
.sev-dot--warning { background: var(--sre-warning); }
.sev-dot--info    { background: var(--sre-info); }
.sev-label { flex: 1; color: var(--sre-text-secondary); }
.sev-count {
  font-weight: var(--sre-fw-bold);
  color: var(--sre-text-primary);
  font-family: var(--sre-font-mono);
  font-feature-settings: "tnum" 1;
}

/* ===== MTTR card ===== */
.mttr-card {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.mttr-card__header {
  justify-content: space-between;
  gap: var(--sre-space-3);
  flex-wrap: wrap;
}
.mttr-card__title-row {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
  color: var(--sre-text-tertiary);
}
.mttr-hero {
  display: flex;
  align-items: stretch;
  gap: 0;
  padding: var(--sre-space-2) 0 var(--sre-space-4);
}
.mttr-hero__divider {
  width: 1px;
  background: var(--sre-border);
  margin: 0 var(--sre-space-4);
  align-self: stretch;
  opacity: 0.6;
}
.mttr-metric {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.mttr-metric__eyebrow {
  font-size: var(--sre-fs-2xs);
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-tertiary);
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
.mttr-metric__value {
  font-size: var(--sre-fs-2xl);
  font-weight: var(--sre-fw-bold);
  color: var(--sre-text-primary);
  line-height: 1.1;
  letter-spacing: -0.015em;
  margin-top: 2px;
}
.mttr-metric--ack .mttr-metric__value { color: var(--sre-warning); }
.mttr-metric--resolve .mttr-metric__value { color: var(--sre-brand-500); }
.mttr-metric__subs {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-secondary);
  flex-wrap: wrap;
}
.mttr-metric__sub em {
  font-style: normal;
  color: var(--sre-text-tertiary);
  font-size: var(--sre-fs-2xs);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  margin-right: 3px;
}
.mttr-metric__dot {
  color: var(--sre-text-tertiary);
  opacity: 0.5;
}
.mttr-metric__count {
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-tertiary);
  margin-top: 2px;
}
.mttr-metric__count .number-display {
  color: var(--sre-text-secondary);
  font-weight: var(--sre-fw-semibold);
  margin-right: 4px;
}

/* ===== MTTR per-severity strip ===== */
.mttr-sev {
  margin-top: auto;
  padding-top: var(--sre-space-3);
  border-top: 1px dashed var(--sre-border);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.mttr-sev__head {
  font-size: var(--sre-fs-2xs);
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-tertiary);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 2px;
}
.mttr-sev__row {
  display: grid;
  grid-template-columns: 64px 1fr 1fr 36px;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  border-radius: var(--sre-radius-sm);
  background: var(--sre-bg-sunken);
  font-size: var(--sre-fs-xs);
  border-left: 2px solid transparent;
}
.mttr-sev__row--critical { border-left-color: var(--sre-critical); }
.mttr-sev__row--warning  { border-left-color: var(--sre-warning); }
.mttr-sev__row--info     { border-left-color: var(--sre-info); }
.mttr-sev__tag {
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-primary);
  font-size: var(--sre-fs-xs);
  text-transform: capitalize;
}
.mttr-sev__metric {
  display: flex;
  align-items: baseline;
  gap: 4px;
  min-width: 0;
}
.mttr-sev__label {
  font-size: var(--sre-fs-2xs);
  color: var(--sre-text-tertiary);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.mttr-sev__val {
  color: var(--sre-text-secondary);
  font-weight: var(--sre-fw-semibold);
  font-size: var(--sre-fs-xs);
}
.mttr-sev__count {
  text-align: right;
  color: var(--sre-text-tertiary);
  font-size: var(--sre-fs-xs);
  font-weight: var(--sre-fw-semibold);
}

/* ===== Gauge ===== */
.gauge-row {
  display: flex;
  align-items: center;
  gap: 0;
}
.gauge-item {
  flex: 1;
  text-align: center;
}
.gauge-divider {
  width: 1px;
  height: 100px;
  background: var(--sre-border);
  margin: 0 var(--sre-space-2);
}
.gauge-label {
  font-size: var(--sre-fs-md);
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-secondary);
  margin-top: -12px;
  letter-spacing: 0.05em;
}
.gauge-sub {
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-tertiary);
  margin-top: 2px;
}

/* ===== Mini stat cards ===== */
.mini-stat-card {
  flex: 1;
  background: var(--sre-bg-card);
  border: 1px solid var(--sre-border);
  border-radius: var(--sre-radius-lg);
  padding: var(--sre-space-4) var(--sre-space-5);
  display: flex;
  align-items: center;
  gap: var(--sre-space-4);
  transition: transform var(--sre-duration-base) var(--sre-ease-out),
              border-color var(--sre-duration-base) var(--sre-ease-out);
}
.mini-stat-card:hover {
  transform: translateY(-2px);
  border-color: var(--sre-border-strong);
}
.mini-stat-card__icon {
  width: 44px;
  height: 44px;
  border-radius: var(--sre-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.04);
}
.mini-stat-label {
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-tertiary);
  font-weight: var(--sre-fw-semibold);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  margin-bottom: var(--sre-space-1);
}
.mini-stat-value {
  font-size: var(--sre-fs-2xl);
  font-weight: var(--sre-fw-bold);
  color: var(--sre-text-primary);
  line-height: 1;
  font-family: var(--sre-font-mono);
  font-feature-settings: "tnum" 1, "lnum" 1;
  letter-spacing: -0.015em;
}

/* ===== Trend header ===== */
.trend-header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: var(--sre-space-3);
}

/* ===== Alert table link ===== */
:deep(.alert-link) {
  color: var(--sre-info);
  cursor: pointer;
  text-decoration: none;
  font-weight: var(--sre-fw-medium);
  transition: color var(--sre-duration-fast) var(--sre-ease-out);
}
:deep(.alert-link:hover) {
  text-decoration: underline;
  color: var(--sre-primary);
}
:deep(.row-critical td) {
  background-color: var(--sre-critical-soft) !important;
  border-left: 2px solid var(--sre-critical);
}
:deep(.row-warning td) {
  background-color: var(--sre-warning-soft) !important;
}
</style>
