<script setup lang="ts">
import { h, ref, reactive, onMounted } from 'vue'
import { useMessage, NTag, NButton, NSpace, NPopconfirm, NSwitch } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { muteRuleApi } from '@/api'
import type { MuteRule } from '@/types'
import { formatTime, kvArrayToRecord } from '@/utils/format'
import { getSeverityType } from '@/utils/alert'
import { AddOutline, RefreshOutline } from '@vicons/ionicons5'
import KVEditor from '@/components/common/KVEditor.vue'
import PageHeader from '@/components/common/PageHeader.vue'

const message = useMessage()
const { t } = useI18n()
const loading = ref(false)
const rules = ref<MuteRule[]>([])
const total = ref(0)
const page = ref(1)

// Modal state
const showModal = ref(false)
const modalTitle = ref('')
const editingId = ref<number | null>(null)
const saving = ref(false)

const defaultForm = {
  name: '',
  description: '',
  match_labels: [] as { key: string; value: string }[],
  severities: [] as string[],
  start_time: null as number | null,
  end_time: null as number | null,
  periodic_start: '',
  periodic_end: '',
  days_of_week: [] as string[],
  timezone: 'Asia/Shanghai',
  rule_ids: '',
  is_enabled: true,
}

const form = reactive({ ...defaultForm })

const severityOptions = [
  { label: () => t('alert.critical'), value: 'critical' },
  { label: () => t('alert.warning'), value: 'warning' },
  { label: () => t('alert.info'), value: 'info' },
]

const daysOfWeekOptions = [
  { label: () => t('mute.monday'), value: '1' },
  { label: () => t('mute.tuesday'), value: '2' },
  { label: () => t('mute.wednesday'), value: '3' },
  { label: () => t('mute.thursday'), value: '4' },
  { label: () => t('mute.friday'), value: '5' },
  { label: () => t('mute.saturday'), value: '6' },
  { label: () => t('mute.sunday'), value: '0' },
]

const timezoneOptions = [
  { label: 'Asia/Shanghai (CST)', value: 'Asia/Shanghai' },
  { label: 'Asia/Tokyo (JST)', value: 'Asia/Tokyo' },
  { label: 'America/New_York (EST)', value: 'America/New_York' },
  { label: 'America/Los_Angeles (PST)', value: 'America/Los_Angeles' },
  { label: 'Europe/London (GMT)', value: 'Europe/London' },
  { label: 'Europe/Berlin (CET)', value: 'Europe/Berlin' },
  { label: 'UTC', value: 'UTC' },
]

function parseSeverities(severitiesStr: string): string[] {
  if (!severitiesStr) return []
  return severitiesStr.split(',').map(s => s.trim()).filter(Boolean)
}

function parseDaysOfWeek(daysStr: string): string[] {
  if (!daysStr) return []
  return daysStr.split(',').map(s => s.trim()).filter(Boolean)
}

const columns = [
  {
    title: () => t('mute.name'),
    key: 'name',
    width: 160,
    ellipsis: { tooltip: true },
    render: (row: MuteRule) =>
      h('div', [
        h('div', { style: 'font-weight: 500' }, row.name),
        row.description
          ? h('div', { style: 'font-size: 11px; color: var(--sre-text-secondary); margin-top: 2px' }, row.description)
          : null,
      ]),
  },
  {
    title: () => t('mute.matchLabels'),
    key: 'match_labels',
    width: 200,
    render: (row: MuteRule) => {
      const labels = row.match_labels || {}
      const entries = Object.entries(labels)
      if (entries.length === 0) return h('span', { style: 'color: var(--sre-text-secondary)' }, '-')
      return h('div', { style: 'display: flex; flex-wrap: wrap; gap: 4px' }, entries.map(([k, v]) =>
        h(NTag, { size: 'small', bordered: false }, { default: () => `${k}=${v}` })
      ))
    },
  },
  {
    title: () => t('mute.severities'),
    key: 'severities',
    width: 160,
    render: (row: MuteRule) => {
      const sevs = parseSeverities(row.severities)
      if (sevs.length === 0) return h('span', { style: 'color: var(--sre-text-secondary)' }, '-')
      return h('div', { style: 'display: flex; gap: 4px; flex-wrap: wrap' }, sevs.map(s =>
        h(NTag, {
          size: 'small',
          type: getSeverityType(s),
          round: true,
        }, { default: () => s })
      ))
    },
  },
  {
    title: () => t('mute.timeRange'),
    key: 'time_range',
    width: 180,
    render: (row: MuteRule) => {
      if (row.start_time && row.end_time) {
        return h('div', { style: 'font-size: 12px' }, [
          h('div', formatTime(row.start_time)),
          h('div', { style: 'color: var(--sre-text-secondary)' }, '→'),
          h('div', formatTime(row.end_time)),
        ])
      }
      return h('span', { style: 'color: var(--sre-text-secondary)' }, '-')
    },
  },
  {
    title: () => t('mute.schedule'),
    key: 'periodic',
    width: 180,
    render: (row: MuteRule) => {
      if (row.periodic_start && row.periodic_end) {
        const days = parseDaysOfWeek(row.days_of_week)
        const dayKeyMap: Record<string, string> = { '0': 'mute.sunday', '1': 'mute.monday', '2': 'mute.tuesday', '3': 'mute.wednesday', '4': 'mute.thursday', '5': 'mute.friday', '6': 'mute.saturday' }
        const dayLabels = days.map(d => {
          const key = dayKeyMap[d]
          return key ? t(key) : d
        })
        return h('div', { style: 'font-size: 12px' }, [
          h('div', `${row.periodic_start} - ${row.periodic_end}`),
          days.length > 0
            ? h('div', { style: 'color: var(--sre-text-secondary); margin-top: 2px' }, dayLabels.join(', '))
            : null,
        ])
      }
      return h('span', { style: 'color: var(--sre-text-secondary)' }, '-')
    },
  },
  {
    title: () => t('common.status'),
    key: 'is_enabled',
    width: 100,
    render: (row: MuteRule) =>
      h(NSwitch, {
        value: row.is_enabled,
        size: 'small',
        onUpdateValue: () => handleToggleEnabled(row),
      }),
  },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 180,
    render: (row: MuteRule) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, {
            size: 'small',
            quaternary: true,
            type: 'info',
            onClick: () => openEdit(row),
          }, { default: () => t('common.edit') }),
          h(NPopconfirm, {
            onPositiveClick: () => handleDelete(row.id),
          }, {
            trigger: () => h(NButton, { size: 'small', quaternary: true, type: 'error' }, { default: () => t('common.delete') }),
            default: () => t('mute.deleteConfirm'),
          }),
        ],
      }),
  },
]

async function fetchRules() {
  loading.value = true
  try {
    const { data } = await muteRuleApi.list({ page: page.value, page_size: 50 })
    rules.value = data.data.list || []
    total.value = data.data.total
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

function resetForm() {
  Object.assign(form, {
    name: '',
    description: '',
    match_labels: [],
    severities: [],
    start_time: null,
    end_time: null,
    periodic_start: '',
    periodic_end: '',
    days_of_week: [],
    timezone: 'Asia/Shanghai',
    rule_ids: '',
    is_enabled: true,
  })
}

function openCreate() {
  editingId.value = null
  modalTitle.value = t('mute.create')
  resetForm()
  showModal.value = true
}

function openEdit(rule: MuteRule) {
  editingId.value = rule.id
  modalTitle.value = t('mute.edit')
  Object.assign(form, {
    name: rule.name,
    description: rule.description || '',
    match_labels: Object.entries(rule.match_labels || {}).map(([key, value]) => ({ key, value })),
    severities: parseSeverities(rule.severities),
    start_time: rule.start_time ? new Date(rule.start_time).getTime() : null,
    end_time: rule.end_time ? new Date(rule.end_time).getTime() : null,
    periodic_start: rule.periodic_start || '',
    periodic_end: rule.periodic_end || '',
    days_of_week: parseDaysOfWeek(rule.days_of_week),
    timezone: rule.timezone || 'Asia/Shanghai',
    rule_ids: rule.rule_ids || '',
    is_enabled: rule.is_enabled,
  })
  showModal.value = true
}

async function handleSave() {
  if (!form.name.trim()) {
    message.warning(t('mute.nameRequired'))
    return
  }

  saving.value = true
  try {
    const payload: Partial<MuteRule> = {
      name: form.name,
      description: form.description,
      match_labels: kvArrayToRecord(form.match_labels),
      severities: form.severities.join(','),
      start_time: form.start_time ? new Date(form.start_time).toISOString() : null,
      end_time: form.end_time ? new Date(form.end_time).toISOString() : null,
      periodic_start: form.periodic_start,
      periodic_end: form.periodic_end,
      days_of_week: form.days_of_week.join(','),
      timezone: form.timezone,
      rule_ids: form.rule_ids,
      is_enabled: form.is_enabled,
    }

    if (editingId.value) {
      await muteRuleApi.update(editingId.value, payload)
      message.success(t('mute.updated'))
    } else {
      await muteRuleApi.create(payload)
      message.success(t('mute.created'))
    }
    showModal.value = false
    fetchRules()
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await muteRuleApi.delete(id)
    message.success(t('mute.deleted'))
    fetchRules()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleToggleEnabled(rule: MuteRule) {
  try {
    await muteRuleApi.update(rule.id, { is_enabled: !rule.is_enabled })
    message.success(rule.is_enabled ? t('mute.disabledSuccess') : t('mute.enabledSuccess'))
    fetchRules()
  } catch (err: any) {
    message.error(err.message)
  }
}

// Preview
const showPreview = ref(false)
const previewLoading = ref(false)
const previewData = ref<Array<{ rule_id: number; rule_name: string; matched_count: number; matched_alerts: any[] }>>([])

async function fetchPreview() {
  previewLoading.value = true
  showPreview.value = true
  try {
    const { data } = await muteRuleApi.preview()
    previewData.value = data.data || []
  } catch (err: any) {
    message.error(err.message)
  } finally {
    previewLoading.value = false
  }
}

onMounted(fetchRules)
</script>

<template>
  <div class="mute-page">
    <PageHeader :title="t('mute.title')" :subtitle="t('mute.subtitle')">
      <template #actions>
        <n-button @click="fetchPreview" :loading="previewLoading">
          {{ t('mute.preview') }}
        </n-button>
        <n-button @click="fetchRules" :loading="loading">
          <template #icon><n-icon :component="RefreshOutline" /></template>
          {{ t('common.refresh') }}
        </n-button>
        <n-button type="primary" @click="openCreate">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('mute.create') }}
        </n-button>
      </template>
    </PageHeader>

    <!-- Mute Preview Panel -->
    <n-card v-if="showPreview" :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px; margin-bottom: 16px">
      <template #header>
        <div style="display: flex; align-items: center; gap: 8px">
          <span>{{ t('mute.previewTitle') }}</span>
          <n-tag size="small" type="info">{{ t('mute.previewNow') }}</n-tag>
        </div>
      </template>
      <n-spin :show="previewLoading">
        <div v-if="previewData.length === 0 && !previewLoading" style="text-align:center; padding: 24px; color: var(--sre-text-secondary)">
          {{ t('mute.previewEmpty') }}
        </div>
        <div v-for="item in previewData" :key="item.rule_id" style="margin-bottom: 16px">
          <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px">
            <strong>{{ item.rule_name }}</strong>
            <n-tag :type="item.matched_count > 0 ? 'warning' : 'success'" size="small" round>
              {{ item.matched_count > 0 ? t('mute.previewMatched', { n: item.matched_count }) : t('mute.previewNoMatch') }}
            </n-tag>
          </div>
          <div v-if="item.matched_alerts.length > 0" style="display: flex; flex-wrap: wrap; gap: 6px; padding-left: 12px">
            <n-tag v-for="ev in item.matched_alerts.slice(0, 10)" :key="ev.id" size="small" bordered>
              #{{ ev.id }} {{ ev.alert_name }}
            </n-tag>
            <n-tag v-if="item.matched_alerts.length > 10" size="small" type="info">+{{ item.matched_alerts.length - 10 }}</n-tag>
          </div>
        </div>
      </n-spin>
    </n-card>

    <n-card :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px">
      <n-data-table
        :loading="loading"
        :columns="columns"
        :data="rules"
        :row-key="(row: MuteRule) => row.id"
        :bordered="false"
        :pagination="{
          page: page,
          pageSize: 50,
          itemCount: total,
          onChange: (p: number) => { page = p; fetchRules() },
        }"
      />

      <n-empty v-if="!loading && rules.length === 0" :description="t('mute.noData')" style="padding: 60px 0">
        <template #extra>
          <n-button type="primary" @click="openCreate">{{ t('mute.createFirst') }}</n-button>
        </template>
      </n-empty>
    </n-card>

    <!-- Create/Edit Modal -->
    <n-modal v-model:show="showModal" preset="card" :title="modalTitle" style="width: 720px" :bordered="false">
      <n-form label-placement="top">
        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('mute.name')" required>
              <n-input v-model:value="form.name" :placeholder="t('mute.name')" />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('mute.severities')">
              <n-select
                v-model:value="form.severities"
                :options="severityOptions"
                multiple
                :placeholder="t('mute.severities')"
              />
            </n-form-item>
          </n-gi>
        </n-grid>

        <n-form-item :label="t('mute.description')">
          <n-input v-model:value="form.description" type="textarea" :placeholder="t('mute.description')" :rows="2" />
        </n-form-item>

        <!-- Match Labels -->
        <n-form-item :label="t('mute.matchLabels')">
          <KVEditor v-model:modelValue="form.match_labels" :add-label="t('mute.addLabel')" />
        </n-form-item>

        <!-- One-time Mute -->
        <n-divider style="margin: 12px 0">{{ t('mute.oneTimeMute') }}</n-divider>
        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('mute.startTime')">
              <n-date-picker
                v-model:value="form.start_time"
                type="datetime"
                clearable
                style="width: 100%"
              />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('mute.endTime')">
              <n-date-picker
                v-model:value="form.end_time"
                type="datetime"
                clearable
                style="width: 100%"
              />
            </n-form-item>
          </n-gi>
        </n-grid>

        <!-- Periodic Mute -->
        <n-divider style="margin: 12px 0">{{ t('mute.periodicMute') }}</n-divider>
        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('mute.periodicStart')">
              <n-time-picker
                v-model:formatted-value="form.periodic_start"
                value-format="HH:mm"
                format="HH:mm"
                clearable
                style="width: 100%"
              />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('mute.periodicEnd')">
              <n-time-picker
                v-model:formatted-value="form.periodic_end"
                value-format="HH:mm"
                format="HH:mm"
                clearable
                style="width: 100%"
              />
            </n-form-item>
          </n-gi>
        </n-grid>

        <n-form-item :label="t('mute.daysOfWeek')">
          <n-checkbox-group v-model:value="form.days_of_week">
            <n-space>
              <n-checkbox v-for="day in daysOfWeekOptions" :key="day.value" :value="day.value" :label="day.label()" />
            </n-space>
          </n-checkbox-group>
        </n-form-item>

        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('mute.timezone')">
              <n-select
                v-model:value="form.timezone"
                :options="timezoneOptions"
                filterable
              />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('mute.ruleIds')">
              <n-input v-model:value="form.rule_ids" placeholder="1,2,3" />
            </n-form-item>
          </n-gi>
        </n-grid>

        <n-form-item :label="t('common.status')">
          <n-switch v-model:value="form.is_enabled">
            <template #checked>{{ t('mute.enabled') }}</template>
            <template #unchecked>{{ t('mute.disabled') }}</template>
          </n-switch>
        </n-form-item>
      </n-form>

      <template #action>
        <n-space justify="end">
          <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="saving" @click="handleSave">
            {{ editingId ? t('common.update') : t('common.create') }}
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.mute-page {
  max-width: 1400px;
}
</style>
