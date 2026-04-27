<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { datasourceApi } from '@/api'
import type { DataSource, QueryResponse } from '@/types'
import { PlayOutline } from '@vicons/ionicons5'
import PageHeader from '@/components/common/PageHeader.vue'

const message = useMessage()
const { t } = useI18n()

const datasources = ref<DataSource[]>([])
const selectedDsId = ref<number | null>(null)
const expression = ref('')
const loading = ref(false)
const fetchLoading = ref(true)
const queryResult = ref<QueryResponse | null>(null)
const queryError = ref('')
const fetchError = ref('')

// Time range for instant query
const timeRangeOptions = [
  { label: 'now', value: 0 },
  { label: '5m ago', value: 300 },
  { label: '15m ago', value: 900 },
  { label: '1h ago', value: 3600 },
  { label: '6h ago', value: 21600 },
  { label: '24h ago', value: 86400 },
]
const timeOffset = ref(0)

async function fetchDatasources() {
  fetchLoading.value = true
  fetchError.value = ''
  try {
    const { data } = await datasourceApi.list({ page: 1, page_size: 100 })
    datasources.value = (data.data.list || []).filter(ds => ds.is_enabled)
  } catch (err: any) {
    fetchError.value = err.message || 'Failed to load datasources'
  } finally {
    fetchLoading.value = false
  }
}

async function handleQuery() {
  if (!selectedDsId.value) { message.warning(t('datasource.selectDatasource')); return }
  if (!expression.value.trim()) { message.warning(t('datasource.queryExpression')); return }

  loading.value = true
  queryResult.value = null
  queryError.value = ''
  try {
    const { data } = await datasourceApi.query(selectedDsId.value, {
      expression: expression.value,
    })
    queryResult.value = data.data
  } catch (err: any) {
    queryError.value = err.message || 'Query failed'
  } finally {
    loading.value = false
  }
}

function formatTimestamp(ts: number) {
  return new Date(ts * 1000).toLocaleString()
}

onMounted(fetchDatasources)
</script>

<template>
  <div class="query-page">
    <PageHeader :title="t('datasource.queryTitle')" :subtitle="t('datasource.querySubtitle')" />

    <!-- Fetch error -->
    <n-alert v-if="fetchError" type="error" style="margin-bottom: 16px" closable @close="fetchError = ''">
      {{ fetchError }}
    </n-alert>

    <!-- No datasources hint -->
    <n-alert v-if="!fetchLoading && !fetchError && datasources.length === 0" type="warning" style="margin-bottom: 16px">
      {{ t('datasource.noEnabledDatasource') }}
    </n-alert>

    <n-card :bordered="false" class="content-card">
      <n-spin :show="fetchLoading">
        <n-space vertical :size="12">
          <n-grid :x-gap="12" :cols="2">
            <n-gi>
              <n-select
                v-model:value="selectedDsId"
                :options="datasources.map(ds => ({ label: `${ds.name} (${ds.type})`, value: ds.id }))"
                :placeholder="t('datasource.selectDatasource')"
                filterable
              />
            </n-gi>
            <n-gi>
              <n-select
                v-model:value="timeOffset"
                :options="timeRangeOptions"
                :placeholder="t('datasource.queryTime')"
              />
            </n-gi>
          </n-grid>

          <n-input
            v-model:value="expression"
            type="textarea"
            :placeholder="t('datasource.queryPlaceholder')"
            :rows="3"
            @keyup.ctrl.enter="handleQuery"
          />

          <n-button
            type="primary"
            :loading="loading"
            @click="handleQuery"
            :disabled="!selectedDsId || !expression.trim()"
          >
            <template #icon><n-icon :component="PlayOutline" /></template>
            {{ t('datasource.executeQuery') }}
          </n-button>
        </n-space>
      </n-spin>
    </n-card>

    <!-- Query error -->
    <n-alert v-if="queryError" type="error" style="margin-top: 16px" closable @close="queryError = ''">
      {{ queryError }}
    </n-alert>

    <!-- Results -->
    <n-card v-if="queryResult" :bordered="false" class="content-card" style="margin-top: 16px">
      <template #header>
        <n-space align="center">
          <span>{{ t('datasource.queryResult') }}</span>
          <n-tag size="small" type="info">{{ queryResult.result_type }}</n-tag>
          <n-tag size="small">{{ queryResult.series?.length ?? queryResult.raw_count ?? 0 }} series</n-tag>
        </n-space>
      </template>

      <template v-if="!queryResult.series || queryResult.series.length === 0">
        <n-empty :description="t('datasource.queryNoResult')" />
      </template>

      <template v-else-if="queryResult.result_type === 'vector' || queryResult.result_type === 'matrix'">
        <n-data-table
          :columns="[
            { title: 'Labels', key: 'labels', minWidth: 200, ellipsis: { tooltip: true } },
            { title: 'Values', key: 'values', minWidth: 300 },
          ]"
          :data="queryResult.series.map((s, i) => ({
            key: i,
            labels: Object.entries(s.labels).map(([k, v]) => `${k}=${v}`).join(', '),
            values: s.values.map(v => `${formatTimestamp(v.ts)}: ${v.value}`).join('\n'),
          }))"
          :max-height="400"
          size="small"
        />
      </template>

      <template v-else>
        <n-code :code="JSON.stringify(queryResult.series, null, 2)" language="json" show-line-numbers />
      </template>
    </n-card>
  </div>
</template>

<style scoped>
.query-page { max-width: 1400px; }
.content-card { border-radius: 12px; }
</style>
