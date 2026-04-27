<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { datasourceApi } from '@/api'
import type { DataSource, DataSourceType } from '@/types'
import { formatTime, kvArrayToRecord } from '@/utils/format'
import { getDatasourceStatusType } from '@/utils/alert'
import { AddOutline, RefreshOutline, CreateOutline } from '@vicons/ionicons5'
import KVEditor from '@/components/common/KVEditor.vue'
import PageHeader from '@/components/common/PageHeader.vue'

const message = useMessage()
const { t } = useI18n()
const loading = ref(false)
const datasources = ref<DataSource[]>([])

// Modal state
const showModal = ref(false)
const modalTitle = ref('')
const editingId = ref<number | null>(null)
const saving = ref(false)

const defaultForm = {
  name: '',
  type: 'prometheus' as DataSourceType,
  endpoint: '',
  description: '',
  auth_type: 'none',
  // Basic auth
  auth_username: '',
  auth_password: '',
  // Bearer token
  auth_token: '',
  // API Key
  auth_key_header: '',
  auth_key_value: '',
  labels: [] as { key: string; value: string }[],
  health_check_interval: 60,
  is_enabled: true,
}

const form = reactive({ ...defaultForm })

const typeOptions = [
  { label: 'Prometheus', value: 'prometheus' },
  { label: 'VictoriaMetrics', value: 'victoriametrics' },
  { label: 'Zabbix', value: 'zabbix' },
  { label: 'VictoriaLogs', value: 'victorialogs' },
]

const authTypeOptions = [
  { label: () => t('datasource.authNone'), value: 'none' },
  { label: () => t('datasource.authBasic'), value: 'basic' },
  { label: () => t('datasource.authBearer'), value: 'bearer' },
  { label: () => t('datasource.authApiKey'), value: 'api_key' },
]

async function fetchList() {
  loading.value = true
  try {
    const { data } = await datasourceApi.list({ page: 1, page_size: 50 })
    datasources.value = data.data.list || []
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  modalTitle.value = t('datasource.add')
  Object.assign(form, {
    name: '',
    type: 'prometheus',
    endpoint: '',
    description: '',
    auth_type: 'none',
    auth_username: '',
    auth_password: '',
    auth_token: '',
    auth_key_header: '',
    auth_key_value: '',
    labels: [],
    health_check_interval: 60,
    is_enabled: true,
  })
  showModal.value = true
}

function openEdit(ds: DataSource) {
  editingId.value = ds.id
  modalTitle.value = t('common.edit')
  Object.assign(form, {
    name: ds.name,
    type: ds.type,
    endpoint: ds.endpoint,
    description: ds.description,
    auth_type: ds.auth_type || 'none',
    // Credential fields are intentionally blank on edit (backend never returns them).
    // Leaving them blank signals "keep existing credentials" to the API.
    auth_username: '',
    auth_password: '',
    auth_token: '',
    auth_key_header: '',
    auth_key_value: '',
    labels: Object.entries(ds.labels || {}).map(([key, value]) => ({ key, value })),
    health_check_interval: ds.health_check_interval || 60,
    is_enabled: ds.is_enabled,
  })
  showModal.value = true
}

async function handleSave() {
  if (!form.name.trim()) {
    message.warning(t('datasource.nameRequired'))
    return
  }
  if (!form.endpoint.trim()) {
    message.warning(t('datasource.endpointRequired'))
    return
  }

  saving.value = true
  try {
    // Build auth_config JSON based on auth_type.
    // For edits: if all credential fields are blank, send empty string to signal "no change".
    let auth_config = ''
    if (form.auth_type === 'basic' && (form.auth_username || form.auth_password)) {
      auth_config = JSON.stringify({ username: form.auth_username, password: form.auth_password })
    } else if (form.auth_type === 'bearer' && form.auth_token) {
      auth_config = JSON.stringify({ token: form.auth_token })
    } else if (form.auth_type === 'api_key' && form.auth_key_value) {
      auth_config = JSON.stringify({ header: form.auth_key_header || 'X-API-Key', value: form.auth_key_value })
    }

    const payload = {
      name: form.name,
      type: form.type,
      endpoint: form.endpoint,
      description: form.description,
      auth_type: form.auth_type,
      auth_config,
      labels: kvArrayToRecord(form.labels),
      health_check_interval: form.health_check_interval,
      is_enabled: form.is_enabled,
    }

    if (editingId.value) {
      await datasourceApi.update(editingId.value, payload)
      message.success(t('datasource.updated'))
    } else {
      await datasourceApi.create(payload)
      message.success(t('datasource.created'))
    }
    showModal.value = false
    fetchList()
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await datasourceApi.delete(id)
    message.success(t('datasource.deleted'))
    fetchList()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleHealthCheck(id: number) {
  try {
    const { data } = await datasourceApi.healthCheck(id)
    const r = data.data
    const latency = r.latency_ms >= 0 ? ` (${r.latency_ms}ms)` : ''
    const version = r.version ? ` · ${r.version}` : ''
    if (r.status === 'healthy') {
      message.success(`✓ ${r.message}${latency}${version}`, { duration: 4000 })
    } else {
      message.error(`✗ ${r.message}${latency}`, { duration: 5000 })
    }
    fetchList()
  } catch (err: any) {
    message.error(err.message)
  }
}

function getTypeColor(type: string) {
  const colors: Record<string, string> = {
    prometheus: '#e6522c',
    victoriametrics: '#621773',
    zabbix: '#d40000',
    victorialogs: '#621773',
  }
  return colors[type] || '#666'
}

function getStatusLabel(status: string) {
  const map: Record<string, string> = {
    healthy: t('datasource.healthy'),
    unhealthy: t('datasource.unhealthy'),
    unknown: t('datasource.unknown'),
  }
  return map[status] || status
}

onMounted(fetchList)
</script>

<template>
  <div class="datasources-page">
    <PageHeader :title="t('datasource.title')" :subtitle="t('datasource.subtitle')">
      <template #actions>
        <n-button @click="fetchList" :loading="loading">
          <template #icon><n-icon :component="RefreshOutline" /></template>
          {{ t('common.refresh') }}
        </n-button>
        <n-button type="primary" @click="openCreate">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('datasource.add') }}
        </n-button>
      </template>
    </PageHeader>

    <n-spin :show="loading">
      <n-grid :x-gap="16" :y-gap="16" :cols="3" responsive="screen">
        <n-gi v-for="(ds, idx) in datasources" :key="ds.id" :style="{ '--sre-stagger-i': idx }">
          <n-card class="ds-card card-hover stagger-item" :bordered="false">
            <div class="ds-header">
              <div class="ds-type-badge" :style="{ background: getTypeColor(ds.type) + '20', color: getTypeColor(ds.type) }">
                {{ ds.type }}
              </div>
              <n-tag :type="getDatasourceStatusType(ds.status)" size="small" round>
                {{ getStatusLabel(ds.status) }}
              </n-tag>
            </div>
            <h3 class="ds-name">{{ ds.name }}</h3>
            <p class="ds-endpoint">{{ ds.endpoint }}</p>
            <p v-if="ds.description" class="ds-desc">{{ ds.description }}</p>

            <!-- Labels display -->
            <div v-if="ds.labels && Object.keys(ds.labels).length > 0" class="ds-labels">
              <n-tag
                v-for="(value, key) in ds.labels"
                :key="key"
                size="small"
                :bordered="false"
                style="background: rgba(128,128,128,0.08)"
              >
                {{ key }}={{ value }}
              </n-tag>
            </div>

            <div class="ds-meta">
              <n-text depth="3" style="font-size: 11px">Auth: {{ ds.auth_type || 'none' }}</n-text>
              <n-text v-if="ds.version" depth="3" style="font-size: 11px">v{{ ds.version }}</n-text>
              <n-text depth="3" style="font-size: 11px">{{ ds.is_enabled ? t('common.enabled') : t('common.disabled') }}</n-text>
            </div>

            <div class="ds-actions">
              <n-button size="small" @click="openEdit(ds)">
                <template #icon><n-icon :component="CreateOutline" :size="14" /></template>
                {{ t('common.edit') }}
              </n-button>
              <n-button size="small" @click="handleHealthCheck(ds.id)">{{ t('datasource.healthCheck') }}</n-button>
              <n-popconfirm @positive-click="handleDelete(ds.id)">
                <template #trigger>
                  <n-button size="small" type="error" quaternary>{{ t('common.delete') }}</n-button>
                </template>
                {{ t('datasource.deleteConfirm') }}
              </n-popconfirm>
            </div>
          </n-card>
        </n-gi>
      </n-grid>

      <n-empty v-if="!loading && datasources.length === 0" :description="t('datasource.noData')" style="padding: 80px 0">
        <template #extra>
          <n-button type="primary" @click="openCreate">{{ t('datasource.addFirst') }}</n-button>
        </template>
      </n-empty>
    </n-spin>

    <!-- Create/Edit Modal -->
    <n-modal v-model:show="showModal" preset="card" :title="modalTitle" style="width: 560px" :bordered="false">
      <n-form label-placement="top">
        <n-form-item :label="t('common.name')" required>
          <n-input v-model:value="form.name" placeholder="e.g. Production VictoriaMetrics" />
        </n-form-item>

        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('common.type')">
              <n-select v-model:value="form.type" :options="typeOptions" />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('datasource.authType')">
              <n-select v-model:value="form.auth_type" :options="authTypeOptions" />
            </n-form-item>
          </n-gi>
        </n-grid>

        <n-form-item :label="t('datasource.endpointUrl')" required>
          <n-input v-model:value="form.endpoint" placeholder="https://vm.example.com:8428" />
        </n-form-item>

        <!-- Basic Auth credentials -->
        <template v-if="form.auth_type === 'basic'">
          <n-grid :x-gap="12" :cols="2">
            <n-gi>
              <n-form-item :label="t('datasource.authUsername')">
                <n-input
                  v-model:value="form.auth_username"
                  :placeholder="editingId ? t('datasource.authCredentialsNote') : t('datasource.authUsername')"
                />
              </n-form-item>
            </n-gi>
            <n-gi>
              <n-form-item :label="t('datasource.authPassword')">
                <n-input
                  v-model:value="form.auth_password"
                  type="password"
                  show-password-on="click"
                  :placeholder="editingId ? t('datasource.authCredentialsNote') : t('datasource.authPassword')"
                />
              </n-form-item>
            </n-gi>
          </n-grid>
        </template>

        <!-- Bearer Token credentials -->
        <template v-if="form.auth_type === 'bearer'">
          <n-form-item :label="t('datasource.authToken')">
            <n-input
              v-model:value="form.auth_token"
              type="password"
              show-password-on="click"
              :placeholder="editingId ? t('datasource.authCredentialsNote') : t('datasource.authToken')"
            />
          </n-form-item>
        </template>

        <!-- API Key credentials -->
        <template v-if="form.auth_type === 'api_key'">
          <n-grid :x-gap="12" :cols="2">
            <n-gi>
              <n-form-item :label="t('datasource.authApiKeyHeader')">
                <n-input
                  v-model:value="form.auth_key_header"
                  :placeholder="t('datasource.authApiKeyHeaderPlaceholder')"
                />
              </n-form-item>
            </n-gi>
            <n-gi>
              <n-form-item :label="t('datasource.authApiKeyValue')">
                <n-input
                  v-model:value="form.auth_key_value"
                  type="password"
                  show-password-on="click"
                  :placeholder="editingId ? t('datasource.authCredentialsNote') : t('datasource.authApiKeyValue')"
                />
              </n-form-item>
            </n-gi>
          </n-grid>
        </template>

        <n-form-item :label="t('common.description')">
          <n-input v-model:value="form.description" type="textarea" :placeholder="t('common.description')" :rows="2" />
        </n-form-item>

        <n-grid :x-gap="12" :cols="2">
          <n-gi>
            <n-form-item :label="t('datasource.healthCheckInterval')">
              <n-input-number v-model:value="form.health_check_interval" :min="10" :max="3600" style="width: 100%" />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('common.enabled')">
              <n-switch v-model:value="form.is_enabled" />
            </n-form-item>
          </n-gi>
        </n-grid>

        <!-- Labels -->
        <n-form-item :label="t('datasource.labels')">
          <KVEditor v-model:modelValue="form.labels" :add-label="t('datasource.addLabel')" />
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
.datasources-page {
  max-width: 1400px;
}

.ds-card {
  background: var(--sre-bg-card);
  border-radius: 12px;
}

.ds-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.ds-type-badge {
  padding: 2px 10px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
}

.ds-name {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: var(--sre-text-primary);
}

.ds-endpoint {
  font-size: 12px;
  color: var(--sre-text-secondary);
  margin: 0 0 8px 0;
  word-break: break-all;
}

.ds-desc {
  font-size: 13px;
  color: var(--sre-text-secondary);
  margin: 0 0 8px 0;
}

.ds-labels {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 8px;
}

.ds-meta {
  display: flex;
  gap: 12px;
  margin-bottom: 8px;
}

.ds-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}
</style>
