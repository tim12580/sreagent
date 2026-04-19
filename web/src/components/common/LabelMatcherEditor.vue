<script setup lang="ts">
import { ref, computed } from 'vue'
import { NAutoComplete, NSelect, NButton, NIcon } from 'naive-ui'
import { AddOutline, CloseOutline } from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'
import { labelRegistryApi } from '@/api'

const { t } = useI18n()

export interface LabelMatcher {
  key: string
  op: string
  value: string
}

const props = withDefaults(defineProps<{
  modelValue: LabelMatcher[]
  datasourceId?: number
  addLabel?: string
}>(), {
  // Fall back to i18n at render time (can't call t() in withDefaults).
  addLabel: '',
})
const resolvedAddLabel = computed(() => props.addLabel || t('labelMatcher.addRow'))

const emit = defineEmits<{
  'update:modelValue': [value: LabelMatcher[]]
}>()

const opOptions = computed(() => [
  { label: `=  (${t('labelMatcher.opEq')})`,      value: '=' },
  { label: `!= (${t('labelMatcher.opNeq')})`,     value: '!=' },
  { label: `=~ (${t('labelMatcher.opRegex')})`,   value: '=~' },
  { label: `!~ (${t('labelMatcher.opNregex')})`,  value: '!~' },
])

// Cache key and value suggestions
const keyOptions = ref<string[]>([])
const valueCache = ref<Record<string, string[]>>({})
const keysLoaded = ref(false)

async function loadKeys() {
  if (keysLoaded.value) return
  try {
    const res = await labelRegistryApi.getKeys(props.datasourceId)
    keyOptions.value = res.data.data || []
    keysLoaded.value = true
  } catch {
    // ignore — autocomplete is best-effort
  }
}

async function loadValues(key: string) {
  if (!key || valueCache.value[key] !== undefined) return
  try {
    const res = await labelRegistryApi.getValues(key, props.datasourceId)
    valueCache.value[key] = res.data.data || []
  } catch {
    valueCache.value[key] = []
  }
}

// Fire-and-forget: autocomplete suggestions are best-effort; void keeps
// lint quiet and signals intent.
void loadKeys()

function addRow() {
  emit('update:modelValue', [...props.modelValue, { key: '', op: '=', value: '' }])
}

function removeRow(i: number) {
  emit('update:modelValue', props.modelValue.filter((_, idx) => idx !== i))
}

function onKeyChange(i: number, key: string) {
  const updated = [...props.modelValue]
  updated[i] = { ...updated[i], key }
  emit('update:modelValue', updated)
  if (key) loadValues(key)
}

function onOpChange(i: number, op: string) {
  const updated = [...props.modelValue]
  updated[i] = { ...updated[i], op }
  emit('update:modelValue', updated)
}

function onValueChange(i: number, value: string) {
  const updated = [...props.modelValue]
  updated[i] = { ...updated[i], value }
  emit('update:modelValue', updated)
}

function keyAutocomplete(key: string) {
  if (!key) return keyOptions.value.map(k => ({ label: k, value: k }))
  return keyOptions.value
    .filter(k => k.toLowerCase().includes(key.toLowerCase()))
    .map(k => ({ label: k, value: k }))
}

function valueAutocomplete(item: LabelMatcher) {
  const vals = valueCache.value[item.key] || []
  if (!item.value) return vals.map(v => ({ label: v, value: v }))
  return vals
    .filter(v => v.toLowerCase().includes(item.value.toLowerCase()))
    .map(v => ({ label: v, value: v }))
}
</script>

<template>
  <div class="lme">
    <div v-for="(item, i) in modelValue" :key="i" class="lme-row">
      <n-auto-complete
        :value="item.key"
        :options="keyAutocomplete(item.key)"
        placeholder="label key"
        size="small"
        class="lme-key"
        :get-show="() => true"
        @update:value="(v: string) => onKeyChange(i, v)"
        @focus="loadKeys"
      />
      <n-select
        :value="item.op"
        :options="opOptions"
        size="small"
        class="lme-op"
        @update:value="(v: string) => onOpChange(i, v)"
      />
      <n-auto-complete
        :value="item.value"
        :options="valueAutocomplete(item)"
        placeholder="value"
        size="small"
        class="lme-val"
        :get-show="() => !!item.key"
        @update:value="(v: string) => onValueChange(i, v)"
        @focus="() => loadValues(item.key)"
      />
      <n-button size="small" quaternary type="error" @click="removeRow(i)">
        <template #icon><n-icon :component="CloseOutline" /></template>
      </n-button>
    </div>
    <n-button dashed size="small" @click="addRow">
      <template #icon><n-icon :component="AddOutline" /></template>
      {{ resolvedAddLabel }}
    </n-button>
  </div>
</template>

<style scoped>
.lme { display: flex; flex-direction: column; gap: 8px; }
.lme-row { display: flex; gap: 6px; align-items: center; }
.lme-key { flex: 2; }
.lme-op { width: 110px; flex-shrink: 0; }
.lme-val { flex: 3; }
</style>
