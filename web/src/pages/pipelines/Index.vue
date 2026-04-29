<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NDataTable, NInput, NSpace, NPopconfirm, NTag, NAlert, useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { pipelineApi } from '@/api'
import type { EventPipeline } from '@/types/pipeline'

const { t } = useI18n()
const router = useRouter()
const message = useMessage()
const loading = ref(false)
const search = ref('')
const list = ref<EventPipeline[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const columns = [
  {
    title: t('pipeline.name'),
    key: 'name',
    ellipsis: { tooltip: true },
  },
  {
    title: t('pipeline.description'),
    key: 'description',
    ellipsis: { tooltip: true },
    render(row: EventPipeline) { return row.description || '-' },
  },
  {
    title: t('pipeline.nodes'),
    key: 'nodes',
    width: 80,
    render(row: EventPipeline) { return row.nodes?.length || 0 },
  },
  {
    title: t('pipeline.status'),
    key: 'disabled',
    width: 100,
    render(row: EventPipeline) {
      return h(NTag, {
        type: row.disabled ? 'warning' : 'success',
        size: 'small',
      }, { default: () => row.disabled ? t('pipeline.disabled') : t('pipeline.active') })
    },
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 200,
    render(row: EventPipeline) {
      return h(NSpace, { size: 'small' }, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => handleEdit(row.id) }, { default: () => t('common.edit') }),
          h(NPopconfirm, { onPositiveClick: () => handleDelete(row.id) }, {
            trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => t('common.delete') }),
            default: () => t('pipeline.deleteConfirm'),
          }),
        ]
      })
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const res = await pipelineApi.list({ page: page.value, page_size: pageSize.value, search: search.value || undefined })
    list.value = res.data.data.list || []
    total.value = res.data.data.total || 0
  } catch (err: any) {
    message.error(err.message || t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await pipelineApi.delete(id)
    message.success(t('pipeline.deleted'))
    fetchList()
  } catch (err: any) {
    message.error(err.message || t('common.deleteFailed'))
  }
}

function handleEdit(id: number) {
  router.push({ name: 'PipelineEditor', params: { id } })
}

function handleCreate() {
  router.push({ name: 'PipelineEditor', params: { id: 'new' } })
}

onMounted(fetchList)
</script>

<template>
  <div style="padding: 20px">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
      <div>
        <h2 style="margin: 0; font-size: 22px; font-weight: 600">{{ t('pipeline.title') }}</h2>
        <p style="margin: 4px 0 0; font-size: 13px; color: var(--sre-text-secondary); max-width: 680px">
          {{ t('pipeline.subtitle') }}
        </p>
      </div>
      <NSpace>
        <NInput
          v-model:value="search"
          :placeholder="t('common.search') + '...'"
          clearable
          style="width: 200px"
          @update:value="fetchList"
        />
        <NButton type="primary" @click="handleCreate">
          + {{ t('pipeline.create') }}
        </NButton>
      </NSpace>
    </div>

    <NDataTable
      :columns="columns"
      :data="list"
      :loading="loading"
      :pagination="{ page, pageSize, itemCount: total, onChange: (p: number) => { page = p; fetchList() } }"
      :row-key="(row: EventPipeline) => row.id"
    >
      <template #empty>
        <div style="padding: 60px 20px; text-align: center">
          <p style="font-size: 15px; color: var(--sre-text-secondary); margin-bottom: 8px">
            {{ t('pipeline.noData') }}
          </p>
          <p style="font-size: 13px; color: var(--sre-text-tertiary, #999); max-width: 500px; margin: 0 auto 16px">
            {{ t('pipeline.noDataHint') }}
          </p>
          <NButton type="primary" @click="handleCreate">
            + {{ t('pipeline.create') }}
          </NButton>
        </div>
      </template>
    </NDataTable>
  </div>
</template>
