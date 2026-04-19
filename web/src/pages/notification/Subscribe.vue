<script setup lang="ts">
import { h, ref, reactive, computed, onMounted } from 'vue'
import { useMessage, NTag, NButton, NSpace, NPopconfirm } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { subscribeRuleApi, notifyRuleApi, userApi, teamApi } from '@/api'
import type { SubscribeRule, NotifyRule, User, Team } from '@/types'
import { AddOutline } from '@vicons/ionicons5'
import { getSeverityType } from '@/utils/alert'
import LabelMatcherEditor from '@/components/common/LabelMatcherEditor.vue'
import type { LabelMatcher } from '@/components/common/LabelMatcherEditor.vue'
import PageHeader from '@/components/common/PageHeader.vue'

const message = useMessage()
const { t } = useI18n()

const loading = ref(false)
const subscriptions = ref<SubscribeRule[]>([])
const showModal = ref(false)
const modalTitle = ref('')
const editingId = ref<number | null>(null)
const saving = ref(false)

// Reference data
const notifyRules = ref<NotifyRule[]>([])
const users = ref<User[]>([])
const teams = ref<Team[]>([])

const form = reactive({
  name: '',
  description: '',
  match_labels: [] as LabelMatcher[],
  severities: [] as string[],
  notify_rule_id: null as number | null,
  subscriber_type: 'user' as 'user' | 'team',
  user_id: null as number | null,
  team_id: null as number | null,
  is_enabled: true,
})

const severityOptions = [
  { label: () => t('alert.critical'), value: 'critical' },
  { label: () => t('alert.warning'), value: 'warning' },
  { label: () => t('alert.info'), value: 'info' },
]

const notifyRuleOptions = computed(() =>
  notifyRules.value.map(r => ({ label: r.name, value: r.id }))
)

const userOptions = computed(() =>
  users.value.map(u => ({ label: u.display_name || u.username, value: u.id }))
)

const teamOptions = computed(() =>
  teams.value.map(t => ({ label: t.name, value: t.id }))
)

function getNotifyRuleName(ruleId: number | null): string {
  if (ruleId == null) return '—'
  const rule = notifyRules.value.find(r => r.id === ruleId)
  return rule?.name || `#${ruleId}`
}

function getSubscriberName(row: SubscribeRule): string {
  if (row.user_id) {
    const user = users.value.find(u => u.id === row.user_id)
    return user ? (user.display_name || user.username) : `User #${row.user_id}`
  }
  if (row.team_id) {
    const team = teams.value.find(t => t.id === row.team_id)
    return team ? team.name : `Team #${row.team_id}`
  }
  return '-'
}

const columns = [
  {
    title: () => t('common.name'),
    key: 'name',
    width: 160,
    ellipsis: { tooltip: true },
  },
  {
    title: () => t('subscribe.matchLabels'),
    key: 'match_labels',
    width: 200,
    render: (row: SubscribeRule) => {
      const labels = row.match_labels || {}
      const entries = Object.entries(labels)
      if (entries.length === 0) return h('span', { style: 'color: #666' }, '-')
      return h(NSpace, { size: 4 }, {
        default: () => entries.map(([k, v]) =>
          h(NTag, { size: 'small', bordered: false }, { default: () => `${k}=${v}` })
        ),
      })
    },
  },
  {
    title: () => t('subscribe.severities'),
    key: 'severities',
    width: 180,
    render: (row: SubscribeRule) => {
      const sevs = (row.severities || '').split(',').filter(Boolean)
      if (sevs.length === 0) return h('span', { style: 'color: #666' }, '-')
      return h(NSpace, { size: 4 }, {
        default: () => sevs.map(s =>
          h(NTag, { size: 'small', type: getSeverityType(s), round: true, bordered: false }, { default: () => s })
        ),
      })
    },
  },
  {
    title: () => t('subscribe.notifyRule'),
    key: 'notify_rule_id',
    width: 140,
    render: (row: SubscribeRule) => getNotifyRuleName(row.notify_rule_id),
  },
  {
    title: () => t('subscribe.subscriber'),
    key: 'subscriber',
    width: 140,
    render: (row: SubscribeRule) => {
      const name = getSubscriberName(row)
      const type = row.user_id ? t('subscribe.user') : t('subscribe.team')
      return h(NSpace, { size: 4, align: 'center' }, {
        default: () => [
          h(NTag, { size: 'small', bordered: false, type: row.user_id ? 'info' : 'success' }, { default: () => type }),
          h('span', {}, name),
        ],
      })
    },
  },
  {
    title: () => t('common.enabled'),
    key: 'is_enabled',
    width: 80,
    render: (row: SubscribeRule) =>
      h(NTag, { type: row.is_enabled ? 'success' : 'default', size: 'small' }, { default: () => row.is_enabled ? t('common.on') : t('common.off') }),
  },
  {
    title: () => t('common.actions'),
    key: 'actions',
    width: 160,
    render: (row: SubscribeRule) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: 'small', quaternary: true, type: 'info', onClick: () => openEdit(row) }, { default: () => t('common.edit') }),
          h(NPopconfirm, { onPositiveClick: () => handleDelete(row.id) }, {
            trigger: () => h(NButton, { size: 'small', quaternary: true, type: 'error' }, { default: () => t('common.delete') }),
            default: () => t('subscribe.deleteConfirm'),
          }),
        ],
      }),
  },
]

async function fetchData() {
  loading.value = true
  try {
    const { data } = await subscribeRuleApi.list({ page: 1, page_size: 100 })
    subscriptions.value = data.data.list || []
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

async function fetchRefData() {
  try {
    const [rulesRes, usersRes, teamsRes] = await Promise.all([
      notifyRuleApi.list({ page: 1, page_size: 100 }),
      userApi.list({ page: 1, page_size: 200 }),
      teamApi.list({ page: 1, page_size: 100 }),
    ])
    notifyRules.value = rulesRes.data.data.list || []
    users.value = usersRes.data.data.list || []
    teams.value = teamsRes.data.data.list || []
  } catch (err: any) {
    message.error(err.message)
  }
}

function resetForm() {
  Object.assign(form, {
    name: '',
    description: '',
    match_labels: [],
    severities: [],
    notify_rule_id: null,
    subscriber_type: 'user',
    user_id: null,
    team_id: null,
    is_enabled: true,
  })
}

function openCreate() {
  editingId.value = null
  modalTitle.value = t('subscribe.create')
  resetForm()
  showModal.value = true
}

function openEdit(row: SubscribeRule) {
  editingId.value = row.id
  modalTitle.value = t('subscribe.edit')
  Object.assign(form, {
    name: row.name,
    description: row.description,
    match_labels: Object.entries(row.match_labels || {}).map(([key, raw]) => {
      for (const op of ['!=', '=~', '!~'] as const) {
        if (raw.startsWith(op)) return { key, op, value: raw.slice(op.length) }
      }
      return { key, op: '=' as const, value: raw }
    }),
    severities: (row.severities || '').split(',').filter(Boolean),
    notify_rule_id: row.notify_rule_id,
    subscriber_type: row.team_id ? 'team' : 'user',
    user_id: row.user_id,
    team_id: row.team_id,
    is_enabled: row.is_enabled,
  })
  showModal.value = true
}

async function handleSave() {
  if (!form.name.trim()) {
    message.warning(t('subscribe.nameRequired'))
    return
  }

  saving.value = true
  try {
    const payload: Partial<SubscribeRule> = {
      name: form.name,
      description: form.description,
      match_labels: Object.fromEntries(form.match_labels.map(m => {
        const v = m.op === '=' ? m.value : `${m.op}${m.value}`
        return [m.key, v]
      })),
      severities: form.severities.join(','),
      // v1.8.1: was `|| 0`, which silently coerced an un-picked value into
       // the numeric id 0 on the backend and then failed the FK constraint.
       // Leave it as null/undefined so the server treats it as "no override".
      notify_rule_id: form.notify_rule_id || null,
      user_id: form.subscriber_type === 'user' ? form.user_id : null,
      team_id: form.subscriber_type === 'team' ? form.team_id : null,
      is_enabled: form.is_enabled,
    }
    if (editingId.value) {
      await subscribeRuleApi.update(editingId.value, payload)
      message.success(t('subscribe.updated'))
    } else {
      await subscribeRuleApi.create(payload)
      message.success(t('subscribe.created'))
    }
    showModal.value = false
    fetchData()
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await subscribeRuleApi.delete(id)
    message.success(t('subscribe.deleted'))
    fetchData()
  } catch (err: any) {
    message.error(err.message)
  }
}

onMounted(() => {
  fetchData()
  fetchRefData()
})
</script>

<template>
  <div class="page-container">
    <PageHeader :title="t('subscribe.title')" :subtitle="t('subscribe.subtitle')">
      <template #actions>
        <n-button type="primary" @click="openCreate">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('subscribe.create') }}
        </n-button>
      </template>
    </PageHeader>

    <n-card :bordered="false" class="content-card">
      <n-data-table
        :loading="loading"
        :columns="columns"
        :data="subscriptions"
        :row-key="(row: SubscribeRule) => row.id"
        :bordered="false"
        size="small"
      />
      <n-empty v-if="!loading && subscriptions.length === 0" :description="t('subscribe.noData')" style="padding: 40px 0" />
    </n-card>

    <!-- Create/Edit Modal -->
    <n-modal v-model:show="showModal" preset="card" :title="modalTitle" style="width: 600px" :bordered="false">
      <n-form label-placement="top">
        <n-form-item :label="t('subscribe.name')" required>
          <n-input v-model:value="form.name" placeholder="e.g. My Critical Alert Sub" />
        </n-form-item>

        <n-form-item :label="t('subscribe.description')">
          <n-input v-model:value="form.description" :placeholder="t('subscribe.description')" />
        </n-form-item>

        <n-form-item :label="t('subscribe.matchLabels')">
          <LabelMatcherEditor v-model:modelValue="form.match_labels" :add-label="t('subscribe.addLabel')" />
        </n-form-item>

        <n-form-item :label="t('subscribe.severities')">
          <n-select
            v-model:value="form.severities"
            :options="severityOptions"
            multiple
            :placeholder="t('common.selectSeverities')"
          />
        </n-form-item>

        <n-form-item :label="t('subscribe.notifyRule')">
          <n-select
            v-model:value="form.notify_rule_id"
            :options="notifyRuleOptions"
            :placeholder="t('subscribe.selectNotifyRule')"
            clearable
          />
        </n-form-item>

        <n-form-item :label="t('subscribe.subscriberType')">
          <n-radio-group v-model:value="form.subscriber_type">
            <n-radio-button value="user">{{ t('subscribe.user') }}</n-radio-button>
            <n-radio-button value="team">{{ t('subscribe.team') }}</n-radio-button>
          </n-radio-group>
        </n-form-item>

        <n-form-item v-if="form.subscriber_type === 'user'" :label="t('subscribe.user')">
          <n-select
            v-model:value="form.user_id"
            :options="userOptions"
            :placeholder="t('subscribe.selectUser')"
            filterable
            clearable
          />
        </n-form-item>

        <n-form-item v-if="form.subscriber_type === 'team'" :label="t('subscribe.team')">
          <n-select
            v-model:value="form.team_id"
            :options="teamOptions"
            :placeholder="t('subscribe.selectTeam')"
            filterable
            clearable
          />
        </n-form-item>

        <n-form-item :label="t('common.enabled')">
          <n-switch v-model:value="form.is_enabled" />
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
.page-container {
  max-width: 1400px;
}

.content-card {
  border-radius: 12px;
}
</style>
