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
import type { DashboardStats, MTTRStats, AlertEvent, EngineStatus, AlertTrendPoint, TopRuleItem } from '@/types'
import { formatTime } from '@/utils/format'
import { getSeverityType, getEventStatusType } from '@/utils/alert'
import PageHeader from '@/components/common/PageHeader.vue'
import {
  AlertCircleOutline,
  ServerOutline,
  CheckmarkCircleOutline,
  ReaderOutline,
  PulseOutline,
  TimeOutline,
  PeopleOutline,
  LayersOutline,
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
const mttrStats = ref<MTTRStats>({
  window_hours: 24,
  mtta_seconds: -1,
  mttr_seconds: -1,
  acked_count: 0,
  resolved_count: 0,
})

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

// ECharts: MTTA gauge
const mttaGaugeOption = computed(() => {
  const val = mttrStats.value.mtta_seconds
  const display = formatDuration(val)
  return makeGaugeOption(display, '#18a058', val >= 0)
})

const mttrGaugeOption = computed(() => {
  const val = mttrStats.value.mttr_seconds
  const display = formatDuration(val)
  return makeGaugeOption(display, '#70c0e8', val >= 0)
})

function makeGaugeOption(label: string, color: string, hasData: boolean) {
  return {
    backgroundColor: 'transparent',
    series: [{
      type: 'gauge',
      radius: '85%',
      startAngle: 210,
      endAngle: -30,
      min: 0,
      max: 100,
      splitNumber: 0,
      progress: {
        show: hasData,
        width: 10,
        itemStyle: { color },
      },
      axisLine: {
        lineStyle: { width: 10, color: [[1, 'rgba(255,255,255,0.08)']] },
      },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: { show: false },
      pointer: { show: false },
      detail: {
        show: true,
        offsetCenter: [0, '10%'],
        formatter: label,
        fontSize: hasData ? 20 : 16,
        fontWeight: 'bold',
        color: hasData ? '#e8e8e8' : '#555',
      },
      data: [{ value: hasData ? 50 : 0 }],
    }],
  }
}

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

onMounted(() => {
  fetchStats()
  fetchEngineStatus()
  fetchRecentAlerts()
  fetchMTTRStats()
  fetchTrendData()
})
</script>

<template>
  <div class="dashboard">
    <PageHeader :title="t('dashboard.title')" :subtitle="t('dashboard.subtitle')" />

    <!-- Top stat cards row -->
    <n-spin :show="loading">
      <n-grid :x-gap="16" :y-gap="16" :cols="5" responsive="screen" style="margin-bottom: 20px">
        <n-gi v-for="card in statCards" :key="card.key">
          <div class="stat-card card-hover">
            <div class="stat-card__accent" :style="{ background: card.gradient }" />
            <div class="stat-card__body">
              <div class="stat-card__icon" :style="{ background: card.color + '18', color: card.color }">
                <n-icon :component="card.icon" :size="22" />
              </div>
              <div class="stat-card__info">
                <div class="stat-card__label">{{ t(card.titleKey) }}</div>
                <div class="stat-card__value">{{ stats[card.key] }}</div>
              </div>
            </div>
          </div>
        </n-gi>

        <!-- Engine status card -->
        <n-gi>
          <div class="stat-card card-hover">
            <div class="stat-card__accent" :style="{ background: engineStatus.running ? 'linear-gradient(135deg,#18a058,#0d6e3e)' : 'linear-gradient(135deg,#e88080,#c0392b)' }" />
            <div class="stat-card__body">
              <div class="stat-card__icon" :style="{ background: (engineStatus.running ? '#18a058' : '#e88080') + '18', color: engineStatus.running ? '#18a058' : '#e88080' }">
                <n-icon :component="PulseOutline" :size="22" />
              </div>
              <div class="stat-card__info">
                <div class="stat-card__label">{{ t('engine.title') }}</div>
                <div class="engine-status-row">
                  <span class="engine-dot" :class="engineStatus.running ? 'engine-dot--running' : 'engine-dot--stopped'" />
                  <span class="stat-card__value" style="font-size:18px">
                    {{ engineStatus.running ? t('engine.running') : t('engine.stopped') }}
                  </span>
                </div>
                <div class="engine-meta">{{ engineStatus.total_rules }} {{ t('engine.rulesUnit') }} · {{ engineStatus.active_alerts }} {{ t('engine.activeUnit') }}</div>
              </div>
            </div>
          </div>
        </n-gi>
      </n-grid>
    </n-spin>

    <!-- Middle row: charts + MTTA/MTTR -->
    <n-grid :x-gap="16" :y-gap="16" :cols="12" style="margin-bottom: 20px">

      <!-- Severity breakdown chart -->
      <n-gi :span="4">
        <div class="panel-card">
          <div class="panel-card__header">
            <span class="panel-card__title">{{ t('dashboard.severityDistribution') }}</span>
          </div>
          <div class="chart-donut-wrap">
            <v-chart :option="severityChartOption" autoresize style="height:180px" />
          </div>
          <div class="sev-legend">
            <div class="sev-item">
              <span class="sev-dot sev-dot--critical" />
              <span class="sev-label">{{ t('alert.critical') }}</span>
              <span class="sev-count">{{ stats.severity_breakdown?.critical ?? 0 }}</span>
            </div>
            <div class="sev-item">
              <span class="sev-dot sev-dot--warning" />
              <span class="sev-label">{{ t('alert.warning') }}</span>
              <span class="sev-count">{{ stats.severity_breakdown?.warning ?? 0 }}</span>
            </div>
            <div class="sev-item">
              <span class="sev-dot sev-dot--info" />
              <span class="sev-label">{{ t('alert.info') }}</span>
              <span class="sev-count">{{ stats.severity_breakdown?.info ?? 0 }}</span>
            </div>
          </div>
        </div>
      </n-gi>

      <!-- MTTA/MTTR gauges -->
      <n-gi :span="5">
        <div class="panel-card" style="height:100%">
          <div class="panel-card__header" style="justify-content:space-between">
            <div style="display:flex;align-items:center;gap:6px">
              <n-icon :component="TimeOutline" size="15" style="color:#aaa" />
              <span class="panel-card__title">MTTA / MTTR</span>
            </div>
            <n-radio-group v-model:value="mttrWindow" size="small" @update:value="fetchMTTRStats">
              <n-radio-button v-for="opt in mttrWindowOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </n-radio-button>
            </n-radio-group>
          </div>
          <n-spin :show="mttrLoading">
            <div class="gauge-row">
              <div class="gauge-item">
                <v-chart :option="mttaGaugeOption" autoresize style="height:140px" />
                <div class="gauge-label">MTTA</div>
                <div class="gauge-sub">{{ mttrStats.acked_count }} {{ t('dashboard.ackedCount') }}</div>
              </div>
              <div class="gauge-divider" />
              <div class="gauge-item">
                <v-chart :option="mttrGaugeOption" autoresize style="height:140px" />
                <div class="gauge-label">MTTR</div>
                <div class="gauge-sub">{{ mttrStats.resolved_count }} {{ t('dashboard.resolvedCount') }}</div>
              </div>
            </div>
          </n-spin>
        </div>
      </n-gi>

      <!-- Users / Teams mini cards -->
      <n-gi :span="3">
        <div style="display:flex;flex-direction:column;gap:16px;height:100%">
          <div class="mini-stat-card">
            <div class="mini-stat-card__icon" style="background:#a855f718;color:#a855f7">
              <n-icon :component="PeopleOutline" :size="20" />
            </div>
            <div>
              <div class="mini-stat-label">{{ t('dashboard.totalUsers') }}</div>
              <div class="mini-stat-value">{{ stats.total_users }}</div>
            </div>
          </div>
          <div class="mini-stat-card">
            <div class="mini-stat-card__icon" style="background:#06b6d418;color:#06b6d4">
              <n-icon :component="LayersOutline" :size="20" />
            </div>
            <div>
              <div class="mini-stat-label">{{ t('dashboard.totalTeams') }}</div>
              <div class="mini-stat-value">{{ stats.total_teams }}</div>
            </div>
          </div>
        </div>
      </n-gi>
    </n-grid>

    <!-- Trend Charts Row -->
    <div class="trend-header">
      <n-radio-group v-model:value="trendDays" size="small" @update:value="fetchTrendData">
        <n-radio-button v-for="opt in trendDayOptions" :key="opt.value" :value="opt.value">
          {{ opt.label() }}
        </n-radio-button>
      </n-radio-group>
    </div>
    <n-spin :show="trendLoading">
      <n-grid :x-gap="16" :y-gap="16" :cols="12" style="margin-bottom: 20px">
        <n-gi :span="8">
          <div class="panel-card">
            <div class="panel-card__header">
              <span class="panel-card__title">{{ t('dashboard.alertTrend') }}</span>
            </div>
            <v-chart :option="trendChartOption" autoresize style="height: 300px" />
          </div>
        </n-gi>
        <n-gi :span="4">
          <div class="panel-card">
            <div class="panel-card__header">
              <span class="panel-card__title">{{ t('dashboard.topRules') }}</span>
            </div>
            <v-chart :option="topRulesChartOption" autoresize style="height: 300px" />
          </div>
        </n-gi>
      </n-grid>
    </n-spin>

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

/* ===== Stat Cards ===== */
.stat-card {
  background: var(--sre-bg-card);
  border: 1px solid var(--sre-border);
  border-radius: var(--sre-radius-lg);
  overflow: hidden;
  position: relative;
  transition: transform var(--sre-duration-base) var(--sre-ease-out),
              box-shadow var(--sre-duration-base) var(--sre-ease-out),
              border-color var(--sre-duration-base) var(--sre-ease-out);
  isolation: isolate;
}
.stat-card::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
  background: radial-gradient(600px circle at 0% 0%,
              rgba(255,255,255,0.04), transparent 40%);
  opacity: 0;
  transition: opacity var(--sre-duration-base) var(--sre-ease-out);
}
.stat-card:hover {
  transform: translateY(-2px);
  border-color: var(--sre-border-strong);
  box-shadow: var(--sre-shadow-md);
}
.stat-card:hover::after { opacity: 1; }
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
