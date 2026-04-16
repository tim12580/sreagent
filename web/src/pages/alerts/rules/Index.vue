<script setup lang="ts">
import { h, ref, reactive, onMounted, computed } from 'vue'
import { useMessage, NTag, NButton, NSpace, NPopconfirm } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { alertRuleApi, datasourceApi } from '@/api'
import type { AlertRule, DataSource, AlertSeverity } from '@/types'
import { formatTime, kvArrayToRecord } from '@/utils/format'
import { getSeverityType, getRuleStatusType } from '@/utils/alert'
import KVEditor from '@/components/common/KVEditor.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { AddOutline, RefreshOutline } from '@vicons/ionicons5'

const message = useMessage()
const { t } = useI18n()
const loading = ref(false)
const rules = ref<AlertRule[]>([])
const total = ref(0)
const page = ref(1)
const datasources = ref<DataSource[]>([])

// Modal state
const showModal = ref(false)
const modalTitle = ref('')
const editingId = ref<number | null>(null)
const saving = ref(false)

const defaultForm = {
  name: '',
  display_name: '',
  description: '',
  datasource_id: null as number | null,
  expression: '',
  for_duration: '5m',
  severity: 'warning' as AlertSeverity,
  labels: [] as { key: string; value: string }[],
  annotations: [] as { key: string; value: string }[],
  group_name: '',
}

const form = reactive({ ...defaultForm })

const severityOptions = [
  { label: () => t('alert.critical'), value: 'critical' },
  { label: () => t('alert.warning'), value: 'warning' },
  { label: () => t('alert.info'), value: 'info' },
]

const datasourceOptions = computed(() =>
  datasources.value.map(ds => ({ label: `${ds.name} (${ds.type})`, value: ds.id }))
)

const selectedDatasource = computed(() =>
  datasources.value.find(ds => ds.id === form.datasource_id)
)

const expressionLang = computed(() => {
  const t = selectedDatasource.value?.type
  if (t === 'victorialogs') return 'LogsQL'
  if (t === 'zabbix') return 'Zabbix'
  return 'PromQL'
})

const expressionPlaceholder = computed(() => {
  const t = selectedDatasource.value?.type
  if (t === 'victorialogs') return 'e.g. error level:error _time:5m'
  if (t === 'zabbix') return 'e.g. system.cpu.util[,user]'
  return 'e.g. avg(rate(cpu_usage_total[5m])) > 0.9'
})

const columns = [
  {
    title: () => t('common.name'),
    key: 'name',
    width: 160,
    ellipsis: { tooltip: true },
    render: (row: AlertRule) =>
      h('div', [
        h('div', { style: 'font-weight: 500' }, row.display_name || row.name),
        h('div', { style: 'font-size: 11px; color: var(--sre-text-secondary)' }, row.name),
      ]),
  },
  {
    title: () => t('alert.groupName'),
    key: 'group_name',
    width: 120,
    ellipsis: { tooltip: true },
  },
  {
    title: () => t('alert.severity'),
    key: 'severity',
    width: 100,
    render: (row: AlertRule) =>
      h(NTag, { type: getSeverityType(row.severity), size: 'small', round: true }, { default: () => row.severity }),
  },
  {
    title: () => t('alert.expression'),
    key: 'expression',
    ellipsis: { tooltip: true },
    render: (row: AlertRule) =>
      h('code', { style: 'font-size: 12px; color: var(--sre-text-secondary)' }, row.expression),
  },
  {
    title: () => t('alert.forDuration'),
    key: 'for_duration',
    width: 90,
  },
  {
    title: () => t('common.status'),
    key: 'status',
    width: 100,
    render: (row: AlertRule) =>
      h(NTag, { type: getRuleStatusType(row.status), size: 'small' }, { default: () => row.status }),
  },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 220,
    render: (row: AlertRule) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, {
            size: 'small',
            quaternary: true,
            type: 'info',
            onClick: () => openEdit(row),
          }, { default: () => t('common.edit') }),
          h(NButton, {
            size: 'small',
            quaternary: true,
            type: row.status === 'enabled' ? 'warning' : 'success',
            onClick: () => handleToggleStatus(row),
          }, { default: () => row.status === 'enabled' ? t('common.disabled') : t('common.enabled') }),
          h(NPopconfirm, {
            onPositiveClick: () => handleDelete(row.id),
          }, {
            trigger: () => h(NButton, { size: 'small', quaternary: true, type: 'error' }, { default: () => t('common.delete') }),
            default: () => t('alert.deleteRuleConfirm'),
          }),
        ],
      }),
  },
]

async function fetchRules() {
  loading.value = true
  try {
    const { data } = await alertRuleApi.list({ page: page.value, page_size: 50 })
    rules.value = data.data.list || []
    total.value = data.data.total
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

async function fetchDatasources() {
  try {
    const { data } = await datasourceApi.list({ page: 1, page_size: 100 })
    datasources.value = data.data.list || []
  } catch (_err) {
    // silently fail
  }
}

function openCreate() {
  editingId.value = null
  modalTitle.value = t('alert.createRule')
  Object.assign(form, {
    name: '',
    display_name: '',
    description: '',
    datasource_id: null,
    expression: '',
    for_duration: '5m',
    severity: 'warning',
    labels: [],
    annotations: [],
    group_name: '',
  })
  showModal.value = true
}

function openEdit(rule: AlertRule) {
  editingId.value = rule.id
  modalTitle.value = t('alert.editRule')
  Object.assign(form, {
    name: rule.name,
    display_name: rule.display_name,
    description: rule.description,
    datasource_id: rule.datasource_id,
    expression: rule.expression,
    for_duration: rule.for_duration,
    severity: rule.severity,
    labels: Object.entries(rule.labels || {}).map(([key, value]) => ({ key, value })),
    annotations: Object.entries(rule.annotations || {}).map(([key, value]) => ({ key, value })),
    group_name: rule.group_name,
  })
  showModal.value = true
}

async function handleSave() {
  if (!form.name.trim()) {
    message.warning(t('alert.nameRequired'))
    return
  }
  if (!form.expression.trim()) {
    message.warning(t('alert.expressionRequired'))
    return
  }

  saving.value = true
  try {
    const payload = {
      name: form.name,
      display_name: form.display_name,
      description: form.description,
      datasource_id: form.datasource_id!,
      expression: form.expression,
      for_duration: form.for_duration,
      severity: form.severity,
      labels: kvArrayToRecord(form.labels),
      annotations: kvArrayToRecord(form.annotations),
      group_name: form.group_name,
    }

    if (editingId.value) {
      await alertRuleApi.update(editingId.value, payload)
      message.success(t('alert.ruleUpdated'))
    } else {
      await alertRuleApi.create(payload)
      message.success(t('alert.ruleCreated'))
    }
    showModal.value = false
    fetchRules()
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

async function handleToggleStatus(rule: AlertRule) {
  const newStatus = rule.status === 'enabled' ? 'disabled' : 'enabled'
  try {
    await alertRuleApi.toggleStatus(rule.id, newStatus)
    message.success(newStatus === 'enabled' ? t('alert.ruleEnabled') : t('alert.ruleDisabled'))
    fetchRules()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleDelete(id: number) {
  try {
    await alertRuleApi.delete(id)
    message.success(t('alert.ruleDeleted'))
    fetchRules()
  } catch (err: any) {
    message.error(err.message)
  }
}

onMounted(() => {
  fetchRules()
  fetchDatasources()
})
</script>

<template>
  <div class="rules-page">
    <PageHeader :title="t('alert.rules')" :subtitle="t('alert.rulesSubtitle')">
      <template #actions>
        <n-button @click="fetchRules" :loading="loading">
          <template #icon><n-icon :component="RefreshOutline" /></template>
          {{ t('common.refresh') }}
        </n-button>
        <n-button type="primary" @click="openCreate">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('alert.createRule') }}
        </n-button>
      </template>
    </PageHeader>

    <n-card :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px">
      <n-data-table
        :loading="loading"
        :columns="columns"
        :data="rules"
        :row-key="(row: AlertRule) => row.id"
        :bordered="false"
        :pagination="{
          page: page,
          pageSize: 50,
          itemCount: total,
          onChange: (p: number) => { page = p; fetchRules() },
        }"
      />

      <n-empty v-if="!loading && rules.length === 0" :description="t('alert.noRules')" style="padding: 60px 0">
        <template #extra>
          <n-button type="primary" @click="openCreate">{{ t('alert.createFirstRule') }}</n-button>
        </template>
      </n-empty>
    </n-card>

    <!-- Create/Edit Modal -->
    <n-modal v-model:show="showModal" preset="card" :title="modalTitle" style="width: 680px" :bordered="false">
      <n-form label-placement="top">
        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('common.name')" required>
              <n-input v-model:value="form.name" placeholder="e.g. high_cpu_usage" />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('alert.displayName')">
              <n-input v-model:value="form.display_name" placeholder="e.g. High CPU Usage" />
            </n-form-item>
          </n-gi>
        </n-grid>

        <n-form-item :label="t('common.description')">
          <n-input v-model:value="form.description" type="textarea" :placeholder="t('common.description')" :rows="2" />
        </n-form-item>

        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('alert.dataSource')">
              <n-select v-model:value="form.datasource_id" :options="datasourceOptions" :placeholder="t('alert.selectDataSource')" clearable />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('alert.groupName')">
              <n-input v-model:value="form.group_name" placeholder="e.g. infrastructure" />
            </n-form-item>
          </n-gi>
        </n-grid>

        <n-form-item required>
          <template #label>
            <n-space size="small" align="center" style="gap:6px">
              <span>{{ t('alert.expression') }}</span>
              <n-tag size="tiny" :type="expressionLang === 'LogsQL' ? 'info' : expressionLang === 'Zabbix' ? 'warning' : 'success'" round>
                {{ expressionLang }}
              </n-tag>
            </n-space>
          </template>
          <n-input
            v-model:value="form.expression"
            type="textarea"
            :placeholder="expressionPlaceholder"
            :rows="3"
            style="font-family: monospace"
          />
        </n-form-item>

        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('alert.forDuration')">
              <n-input v-model:value="form.for_duration" placeholder="e.g. 5m, 10m, 1h" />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('alert.severity')">
              <n-select v-model:value="form.severity" :options="severityOptions" />
            </n-form-item>
          </n-gi>
        </n-grid>

        <!-- Labels -->
        <n-form-item :label="t('alert.labels')">
          <KVEditor v-model:modelValue="form.labels" :add-label="t('alert.addLabel')" />
        </n-form-item>

        <!-- Annotations -->
        <n-form-item :label="t('alert.annotations')">
          <KVEditor v-model:modelValue="form.annotations" :add-label="t('alert.addAnnotation')" key-placeholder="Key (e.g. summary)" />
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
.rules-page {
  max-width: 1400px;
}
</style>
