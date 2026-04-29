<script setup lang="ts">
import { h, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NDataTable, NInput, NSpace, NPopconfirm } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { dashboardV2Api } from '@/api'
import type { DashboardV2 } from '@/types/dashboard'
import PageHeader from '@/components/common/PageHeader.vue'
import { AddOutline } from '@vicons/ionicons5'

const router = useRouter()
const message = useMessage()
const { t } = useI18n()
const loading = ref(false)
const search = ref('')
const list = ref<DashboardV2[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const columns = [
  {
    title: () => t('common.name'),
    key: 'name',
    ellipsis: { tooltip: true },
    render(row: DashboardV2) {
      return h('a', {
        class: 'dash-link',
        onClick: () => handleEdit(row.id),
      }, row.name)
    },
  },
  { title: () => t('common.description'), key: 'description', ellipsis: { tooltip: true } },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 160,
    render(row: DashboardV2) {
      return h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => handleEdit(row.id) }, { default: () => t('common.edit') }),
          h(NPopconfirm, { onPositiveClick: () => handleDelete(row.id) }, {
            trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { default: () => t('common.delete') }),
            default: () => t('common.confirmDelete'),
          }),
        ],
      })
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const res = await dashboardV2Api.list({ page: page.value, page_size: pageSize.value, search: search.value || undefined })
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
    await dashboardV2Api.delete(id)
    message.success(t('dashboardV2.deleted'))
    fetchList()
  } catch (err: any) {
    message.error(err.message || t('common.deleteFailed'))
  }
}

function handleEdit(id: number) {
  router.push({ name: 'DashboardV2View', params: { id } })
}

onMounted(fetchList)
</script>

<template>
  <div class="dash-list-page">
    <PageHeader :title="t('dashboardV2.title')" :subtitle="t('dashboardV2.subtitle')">
      <template #actions>
        <NInput
          v-model:value="search"
          :placeholder="t('common.search')"
          clearable
          style="width: 200px"
          @update:value="fetchList"
        />
        <NButton type="primary" @click="router.push({ name: 'DashboardV2View', params: { id: 'new' } })">
          <template #icon>
            <AddOutline />
          </template>
          {{ t('dashboardV2.newDashboard') }}
        </NButton>
      </template>
    </PageHeader>

    <NDataTable
      :columns="columns"
      :data="list"
      :loading="loading"
      :pagination="{ page, pageSize, itemCount: total, onChange: (p: number) => { page = p; fetchList() } }"
      :row-key="(row: DashboardV2) => row.id"
    >
      <template #empty>
        <div class="empty-state">
          {{ t('dashboardV2.emptyHint') }}
        </div>
      </template>
    </NDataTable>
  </div>
</template>

<style scoped>
.dash-list-page {
  max-width: 1400px;
}
.dash-link {
  color: var(--sre-primary);
  cursor: pointer;
}
.dash-link:hover {
  text-decoration: underline;
}
.empty-state {
  padding: 48px 0;
  text-align: center;
  color: var(--sre-text-tertiary);
}
</style>
