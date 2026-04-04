<script setup lang="ts">
import { h, ref, reactive, computed, onMounted } from 'vue'
import { useMessage, NTag, NButton, NSpace, NPopconfirm } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { alertChannelApi, notifyMediaApi, messageTemplateApi } from '@/api'
import type { AlertChannel, NotifyMedia, MessageTemplate } from '@/types'
import { AddOutline } from '@vicons/ionicons5'
import { getSeverityType } from '@/utils/alert'
import KVEditor from '@/components/common/KVEditor.vue'
import PageHeader from '@/components/common/PageHeader.vue'

const message = useMessage()
const { t } = useI18n()

const loading = ref(false)
const channels = ref<AlertChannel[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const showModal = ref(false)
const modalTitle = ref('')
const editingId = ref<number | null>(null)
const saving = ref(false)
const testingId = ref<number | null>(null)

const mediaList = ref<NotifyMedia[]>([])
const templateList = ref<MessageTemplate[]>([])

// Severity options
const severityOptions = [
  { label: 'Critical', value: 'critical' },
  { label: 'Warning', value: 'warning' },
  { label: 'Info', value: 'info' },
]

const form = reactive({
  name: '',
  description: '',
  match_labels: [] as { key: string; value: string }[],
  severities: [] as string[],
  media_id: null as number | null,
  template_id: null as number | null,
  throttle_min: 5,
  is_enabled: true,
})

const mediaOptions = computed(() =>
  mediaList.value.map((m) => ({ label: m.name, value: m.id }))
)

const templateOptions = computed(() => [
  { label: t('common.noData') + ' (默认)', value: null as any },
  ...templateList.value.map((tp) => ({ label: tp.name, value: tp.id })),
])

function severityBadges(severitiesStr: string) {
  if (!severitiesStr) return []
  return severitiesStr.split(',').map((s) => s.trim()).filter(Boolean)
}

function labelEntries(matchLabels: Record<string, string>) {
  return Object.entries(matchLabels || {})
}

const columns = [
  {
    title: () => t('common.name'),
    key: 'name',
    minWidth: 140,
    render: (row: AlertChannel) =>
      h('span', { style: 'font-weight: 500' }, row.name),
  },
  {
    title: () => t('alertChannel.matchLabels'),
    key: 'match_labels',
    minWidth: 140,
    render: (row: AlertChannel) => {
      const entries = labelEntries(row.match_labels)
      if (!entries.length) return h('span', { style: 'color: var(--sre-text-secondary); font-size: 12px' }, '-')
      return h(NSpace, { size: 4, wrap: true }, {
        default: () => entries.map(([k, v]) =>
          h(NTag, { size: 'small', type: 'default' }, { default: () => `${k}=${v}` })
        ),
      })
    },
  },
  {
    title: () => t('alertChannel.severities'),
    key: 'severities',
    width: 160,
    render: (row: AlertChannel) => {
      const badges = severityBadges(row.severities)
      if (!badges.length) return h('span', { style: 'color: var(--sre-text-secondary); font-size: 12px' }, t('alert.all'))
      return h(NSpace, { size: 4 }, {
        default: () => badges.map((s) =>
          h(NTag, { size: 'small', type: getSeverityType(s), round: true }, { default: () => s.toUpperCase() })
        ),
      })
    },
  },
  {
    title: () => t('alertChannel.mediaLabel'),
    key: 'media_id',
    width: 140,
    render: (row: AlertChannel) => {
      const media = mediaList.value.find((m) => m.id === row.media_id)
      return h('span', { style: 'font-size: 13px' }, media?.name || `#${row.media_id}`)
    },
  },
  {
    title: () => t('alertChannel.throttle'),
    key: 'throttle_min',
    width: 110,
    render: (row: AlertChannel) =>
      h('span', { style: 'font-size: 13px' }, `${row.throttle_min} min`),
  },
  {
    title: () => t('common.status'),
    key: 'is_enabled',
    width: 90,
    render: (row: AlertChannel) =>
      h(NTag, {
        size: 'small',
        type: row.is_enabled ? 'success' : 'default',
      }, { default: () => row.is_enabled ? t('common.enabled') : t('common.disabled') }),
  },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row: AlertChannel) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, {
            size: 'tiny',
            secondary: true,
            loading: testingId.value === row.id,
            onClick: () => handleTest(row.id),
          }, { default: () => t('alertChannel.testSend') }),
          h(NButton, {
            size: 'tiny',
            type: 'primary',
            secondary: true,
            onClick: () => openEdit(row),
          }, { default: () => t('common.edit') }),
          h(NPopconfirm, {
            onPositiveClick: () => handleDelete(row.id),
          }, {
            trigger: () =>
              h(NButton, { size: 'tiny', type: 'error', secondary: true }, { default: () => t('common.delete') }),
            default: () => t('alertChannel.deleteConfirm'),
          }),
        ],
      }),
  },
]

async function fetchChannels() {
  loading.value = true
  try {
    const { data } = await alertChannelApi.list({ page: page.value, page_size: pageSize.value })
    channels.value = data.data.list || []
    total.value = data.data.total
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

async function fetchMedia() {
  try {
    const { data } = await notifyMediaApi.list({ page: 1, page_size: 200 })
    mediaList.value = data.data.list || []
  } catch {
    // ignore
  }
}

async function fetchTemplates() {
  try {
    const { data } = await messageTemplateApi.list({ page: 1, page_size: 200 })
    templateList.value = data.data.list || []
  } catch {
    // ignore
  }
}

function resetForm() {
  form.name = ''
  form.description = ''
  form.match_labels = []
  form.severities = []
  form.media_id = null
  form.template_id = null
  form.throttle_min = 5
  form.is_enabled = true
}

function openCreate() {
  editingId.value = null
  resetForm()
  modalTitle.value = t('alertChannel.create')
  showModal.value = true
}

function openEdit(row: AlertChannel) {
  editingId.value = row.id
  form.name = row.name
  form.description = row.description
  form.match_labels = Object.entries(row.match_labels || {}).map(([key, value]) => ({ key, value }))
  form.severities = row.severities ? row.severities.split(',').map((s) => s.trim()).filter(Boolean) : []
  form.media_id = row.media_id
  form.template_id = row.template_id
  form.throttle_min = row.throttle_min
  form.is_enabled = row.is_enabled
  modalTitle.value = t('common.edit')
  showModal.value = true
}

function buildPayload() {
  const matchLabels: Record<string, string> = {}
  form.match_labels.forEach(({ key, value }) => {
    if (key.trim()) matchLabels[key.trim()] = value
  })
  return {
    name: form.name,
    description: form.description,
    match_labels: matchLabels,
    severities: form.severities.join(','),
    media_id: form.media_id as number,
    template_id: form.template_id,
    throttle_min: form.throttle_min,
    is_enabled: form.is_enabled,
  }
}

async function handleSave() {
  if (!form.name.trim()) {
    message.warning(t('alertChannel.nameRequired'))
    return
  }
  if (!form.media_id) {
    message.warning(t('alertChannel.mediaRequired'))
    return
  }
  saving.value = true
  try {
    const payload = buildPayload()
    if (editingId.value) {
      await alertChannelApi.update(editingId.value, payload)
      message.success(t('alertChannel.updated'))
    } else {
      await alertChannelApi.create(payload)
      message.success(t('alertChannel.created'))
    }
    showModal.value = false
    fetchChannels()
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await alertChannelApi.delete(id)
    message.success(t('alertChannel.deleted'))
    fetchChannels()
  } catch (err: any) {
    message.error(err.message)
  }
}

async function handleTest(id: number) {
  testingId.value = id
  try {
    await alertChannelApi.test(id)
    message.success(t('alertChannel.testSuccess'))
  } catch (err: any) {
    message.error(err.message || t('alertChannel.testFailed'))
  } finally {
    testingId.value = null
  }
}

onMounted(() => {
  fetchChannels()
  fetchMedia()
  fetchTemplates()
})
</script>

<template>
  <div class="alert-channels-page">
    <!-- Header -->
    <PageHeader :title="t('alertChannel.title')" :subtitle="t('alertChannel.subtitle')">
      <template #actions>
        <n-button type="primary" @click="openCreate">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('alertChannel.create') }}
        </n-button>
      </template>
    </PageHeader>

    <!-- Table -->
    <n-card :bordered="false" style="background: var(--sre-bg-card); border-radius: 12px">
      <n-data-table
        :loading="loading"
        :columns="columns"
        :data="channels"
        :row-key="(row: AlertChannel) => row.id"
        :bordered="false"
        :pagination="{
          page: page,
          pageSize: pageSize,
          itemCount: total,
          showSizePicker: true,
          pageSizes: [20, 50, 100],
          onChange: (p: number) => { page = p; fetchChannels() },
          onUpdatePageSize: (s: number) => { pageSize = s; page = 1; fetchChannels() },
        }"
      />
    </n-card>

    <!-- Create / Edit Modal -->
    <n-modal
      v-model:show="showModal"
      :title="modalTitle"
      preset="card"
      style="width: 560px"
      :bordered="false"
    >
      <n-form label-placement="left" label-width="100" size="medium">
        <n-form-item :label="t('common.name')" required>
          <n-input v-model:value="form.name" :placeholder="t('alertChannel.nameRequired')" clearable />
        </n-form-item>

        <n-form-item :label="t('common.description')">
          <n-input v-model:value="form.description" type="textarea" :rows="2" clearable />
        </n-form-item>

        <!-- Match Labels -->
        <n-form-item :label="t('alertChannel.matchLabels')">
          <KVEditor v-model:modelValue="form.match_labels" :add-label="t('alertChannel.addLabel')" />
        </n-form-item>

        <!-- Severities -->
        <n-form-item :label="t('alertChannel.severities')">
          <n-select
            v-model:value="form.severities"
            :options="severityOptions"
            multiple
            :placeholder="t('common.selectSeverities')"
            clearable
            style="width: 100%"
          />
        </n-form-item>

        <!-- Notify Media -->
        <n-form-item :label="t('alertChannel.mediaLabel')" required>
          <n-select
            v-model:value="form.media_id"
            :options="mediaOptions"
            :placeholder="t('alertChannel.mediaRequired')"
            clearable
            style="width: 100%"
          />
        </n-form-item>

        <!-- Template -->
        <n-form-item :label="t('alertChannel.template')">
          <n-select
            v-model:value="form.template_id"
            :options="templateOptions"
            clearable
            style="width: 100%"
          />
        </n-form-item>

        <!-- Throttle -->
        <n-form-item :label="t('alertChannel.throttle')">
          <n-input-number v-model:value="form.throttle_min" :min="0" :max="10080" style="width: 160px" />
        </n-form-item>

        <!-- Enabled -->
        <n-form-item :label="t('common.enabled')">
          <n-switch v-model:value="form.is_enabled" />
        </n-form-item>
      </n-form>

      <template #footer>
        <div style="display: flex; justify-content: flex-end; gap: 8px">
          <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="saving" @click="handleSave">
            {{ t('common.save') }}
          </n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.alert-channels-page {
  max-width: 1400px;
}
</style>
