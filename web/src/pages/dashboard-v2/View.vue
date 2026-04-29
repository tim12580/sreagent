<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NSpace, NInput, useMessage, NModal, NPopconfirm } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { dashboardV2Api, datasourceApi } from '@/api'
import type { DashboardV2, DashboardConfig, PanelConfig, VariableConfig } from '@/types/dashboard'
import type { DataSource } from '@/types'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQueryEngine, createDefaultTarget } from '@/composables/useQueryEngine'
import { useVariable } from '@/composables/useVariable'
import TimeRangePicker from '@/components/time/TimeRangePicker.vue'
import RefreshPicker from '@/components/time/RefreshPicker.vue'
import QueryPanel from '@/components/query/QueryPanel.vue'
import QueryResultChart from '@/components/query/QueryResultChart.vue'
import PanelCard from '@/components/query/PanelCard.vue'
import { ArrowBackOutline, AddOutline } from '@vicons/ionicons5'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { t } = useI18n()

const isNew = computed(() => route.params.id === 'new')
const dashboard = ref<DashboardV2 | null>(null)
const loading = ref(false)
const saving = ref(false)
const config = ref<DashboardConfig>({
  panels: [],
  layout: { cols: 24, rowHeight: 100 },
  variables: [],
})

const datasources = ref<DataSource[]>([])

const {
  timeRange,
  isRelative,
  relativeDuration,
  autoRefreshInterval,
  setRelative,
  setAbsolute,
} = useTimeRange('1h')

const {
  targets,
  globalLoading,
  addTarget,
  removeTarget,
  toggleTarget,
  updateTarget,
  executeAll,
  executeQuery,
} = useQueryEngine(timeRange)

const variableConfig = ref<VariableConfig[]>(config.value.variables || [])
const { variableList, replaceVariables, setValue, resolveAll } = useVariable(variableConfig, timeRange)

// --- Panel management ---
const panelToDelete = ref<PanelConfig | null>(null)

function addPanelFromQuery(type: PanelConfig['type'] = 'timeseries') {
  const activeTargets = targets.value.filter(t => t.enabled && t.datasourceId && t.expression?.trim())
  if (!activeTargets.length) {
    message.warning(t('dashboardV2.noQueryToAdd') || 'Enter a query first')
    return
  }
  const panel: PanelConfig = {
    id: `panel-${Date.now()}`,
    title: `Panel ${config.value.panels.length + 1}`,
    type,
    gridPos: { x: 0, y: config.value.panels.length * 6, w: 24, h: 6 },
    targets: activeTargets.map(t => ({
      datasourceId: t.datasourceId!,
      expression: t.expression,
      legendFormat: t.legendFormat || '',
    })),
    options: {},
  }
  config.value.panels.push(panel)
  message.success(t('dashboardV2.panelAdded') || 'Panel added')
}

function removePanel(id: string) {
  config.value.panels = config.value.panels.filter(p => p.id !== id)
}

function updatePanelTitle(id: string, title: string) {
  const p = config.value.panels.find(p => p.id === id)
  if (p) p.title = title
}

// --- Data ---
async function fetchDatasources() {
  try {
    const res = await datasourceApi.list({ page: 1, page_size: 100 })
    datasources.value = (res.data.data.list || []).filter((ds: any) => ds.is_enabled)
  } catch { /* ignore */ }
}

async function fetchDashboard() {
  if (isNew.value) return
  loading.value = true
  try {
    const res = await dashboardV2Api.get(Number(route.params.id))
    dashboard.value = res.data.data
    if (dashboard.value.config) {
      try {
        config.value = JSON.parse(dashboard.value.config)
        // Ensure panels array exists
        if (!config.value.panels) config.value.panels = []
        if (!config.value.layout) config.value.layout = { cols: 24, rowHeight: 100 }
        variableConfig.value = config.value.variables || []
      } catch { /* ignore */ }
    }
  } catch (err: any) {
    message.error(err.message || t('common.loadFailed'))
    router.back()
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const cfg = { ...config.value, variables: variableConfig.value }
    const data = {
      name: dashboard.value?.name || 'Untitled',
      description: dashboard.value?.description || '',
      tags: dashboard.value?.tags || {},
      config: JSON.stringify(cfg),
      is_public: dashboard.value?.is_public || false,
    }
    if (isNew.value) {
      const res = await dashboardV2Api.create(data)
      message.success(t('dashboardV2.created'))
      router.replace({ name: 'DashboardV2View', params: { id: res.data.data.id } })
    } else if (dashboard.value) {
      await dashboardV2Api.update(dashboard.value.id, data)
      message.success(t('dashboardV2.saved'))
    }
  } catch (err: any) {
    message.error(err.message || t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

function handleExecuteSingle(id: string) {
  const target = targets.value.find(t => t.id === id)
  if (target) executeQuery(target)
}

const hasPanels = computed(() => config.value.panels.length > 0)
const hasResults = computed(() => targets.value.some(t => t.series && t.series.length > 0))

onMounted(() => {
  fetchDatasources()
  fetchDashboard()
})
</script>

<template>
  <div class="dashboard-view">
    <!-- Header -->
    <div class="dashboard-header">
      <div class="header-left">
        <NButton quaternary size="small" @click="router.push({ name: 'DashboardV2List' })">
          <template #icon><ArrowBackOutline /></template>
          {{ t('dashboardV2.back') }}
        </NButton>
        <NInput
          v-if="dashboard || isNew"
          :value="dashboard?.name || ''"
          :placeholder="t('dashboardV2.name')"
          size="small"
          style="width: 280px"
          @update:value="(v: string) => { if (dashboard) dashboard.name = v; else dashboard = { name: v } as any }"
        />
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
          @update:value="(v) => autoRefreshInterval = v"
        />
        <NButton type="primary" size="small" :loading="saving" @click="handleSave">
          {{ t('dashboardV2.save') }}
        </NButton>
      </div>
    </div>

    <!-- Variable bar -->
    <div v-if="variableList.length > 0" class="variable-bar">
      <div v-for="v in variableList" :key="v.config.name" class="var-item">
        <label>{{ v.config.label || v.config.name }}</label>
        <NSelect
          v-if="v.config.type === 'query' || v.config.type === 'custom'"
          :value="v.value"
          :options="v.options.map(o => ({ label: o, value: o }))"
          :loading="v.loading"
          size="small"
          style="width: 160px"
          @update:value="(val: string) => setValue(v.config.name, val)"
        />
        <NInput
          v-else-if="v.config.type === 'textbox'"
          :value="v.value"
          size="small"
          style="width: 160px"
          @update:value="(val: string) => setValue(v.config.name, val)"
        />
        <span v-else class="var-value">{{ v.value }}</span>
      </div>
    </div>

    <!-- PANEL GRID -->
    <div v-if="hasPanels" class="panel-grid">
      <div
        v-for="panel in config.panels"
        :key="panel.id"
        class="panel-grid-item"
        :style="{
          gridColumn: `${(panel.gridPos?.x || 0) + 1} / span ${panel.gridPos?.w || 24}`,
          gridRow: `${(panel.gridPos?.y || 0) + 1} / span ${panel.gridPos?.h || 6}`,
        }"
      >
        <div class="panel-toolbar">
          <NInput
            :value="panel.title"
            size="tiny"
            style="width: 180px"
            @update:value="(v: string) => updatePanelTitle(panel.id, v)"
          />
          <NSpace :size="4">
            <NButton quaternary size="tiny" @click="removePanel(panel.id)">&times;</NButton>
          </NSpace>
        </div>
        <PanelCard :panel="panel" :time-range="timeRange" />
      </div>
    </div>

    <!-- Empty state -->
    <div v-if="!hasPanels && !hasResults" class="empty-dashboard">
      <div class="empty-text">{{ t('dashboardV2.emptyDashboardHint') || 'Add panels from queries below to build your dashboard' }}</div>
    </div>

    <!-- Query editor (always visible) -->
    <details class="query-editor-section" :open="!hasPanels">
      <summary class="query-editor-toggle">{{ t('dashboardV2.queryEditor') || 'Query Editor' }}</summary>
      <QueryPanel
        :targets="targets"
        :datasources="datasources"
        :loading="globalLoading"
        @add="addTarget"
        @remove="removeTarget"
        @toggle="toggleTarget"
        @update="updateTarget"
        @execute="handleExecuteSingle"
        @execute-all="executeAll"
      />

      <!-- Query results + add panel buttons -->
      <div v-if="hasResults" class="query-results-section">
        <div class="results-actions">
          <span class="results-label">{{ t('dashboardV2.addAsPanel') || 'Add as panel:' }}</span>
          <NSpace size="small">
            <NButton size="tiny" secondary @click="addPanelFromQuery('timeseries')">{{ t('dashboardV2.panelTimeseries') || 'Chart' }}</NButton>
            <NButton size="tiny" secondary @click="addPanelFromQuery('stat')">{{ t('dashboardV2.panelStat') || 'Stat' }}</NButton>
            <NButton size="tiny" secondary @click="addPanelFromQuery('table')">{{ t('dashboardV2.panelTable') || 'Table' }}</NButton>
          </NSpace>
        </div>
        <QueryResultChart :targets="targets" :time-range="timeRange" :height="300" />
      </div>
    </details>
  </div>
</template>

<style scoped>
.dashboard-view {
  padding: 20px;
  max-width: 1600px;
}
.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Variable bar */
.variable-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
  padding: 12px;
  background: var(--sre-bg-card);
  border: 1px solid var(--sre-border);
  border-radius: 8px;
}
.var-item {
  display: flex;
  align-items: center;
  gap: 6px;
}
.var-item label {
  font-size: 12px;
  color: var(--sre-text-secondary);
  white-space: nowrap;
}
.var-value {
  font-size: 13px;
  padding: 4px 8px;
  background: var(--sre-bg-sunken);
  border-radius: 4px;
  color: var(--sre-text-primary);
}

/* Panel grid */
.panel-grid {
  display: grid;
  grid-template-columns: repeat(24, 1fr);
  gap: 12px;
  margin-bottom: 20px;
  min-height: 0;
}
.panel-grid-item {
  display: flex;
  flex-direction: column;
  min-height: 200px;
}
.panel-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
  padding: 0 2px;
}

/* Empty dashboard */
.empty-dashboard {
  padding: 60px 0;
  text-align: center;
}
.empty-text {
  font-size: 14px;
  color: var(--sre-text-tertiary);
}

/* Query editor */
.query-editor-section {
  border: 1px solid var(--sre-border);
  border-radius: 8px;
  padding: 12px 16px;
  background: var(--sre-bg-sunken);
}
.query-editor-toggle {
  font-size: 13px;
  font-weight: 600;
  color: var(--sre-text-secondary);
  cursor: pointer;
  user-select: none;
}
.query-editor-toggle:hover {
  color: var(--sre-text-primary);
}
.query-results-section {
  margin-top: 12px;
  border-top: 1px solid var(--sre-border);
  padding-top: 12px;
}
.results-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.results-label {
  font-size: 12px;
  color: var(--sre-text-secondary);
}
</style>
