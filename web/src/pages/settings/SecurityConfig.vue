<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { securitySettingsApi } from '@/api'

const message = useMessage()
const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)

const jwtExpireSeconds = ref(86400)

const expireOptions = [
  { label: '1 小时 / 1 Hour', value: 3600 },
  { label: '4 小时 / 4 Hours', value: 14400 },
  { label: '8 小时 / 8 Hours', value: 28800 },
  { label: '24 小时 / 24 Hours', value: 86400 },
  { label: '7 天 / 7 Days', value: 604800 },
]

async function fetchConfig() {
  loading.value = true
  try {
    const { data } = await securitySettingsApi.getConfig()
    jwtExpireSeconds.value = data.data.jwt_expire_seconds
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await securitySettingsApi.updateConfig({ jwt_expire_seconds: jwtExpireSeconds.value })
    message.success(t('common.savedSuccess'))
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

onMounted(fetchConfig)
</script>

<template>
  <n-spin :show="loading">
    <n-form label-placement="top" style="max-width: 480px">
      <n-form-item :label="t('settings.jwtExpireTime')">
        <n-select
          v-model:value="jwtExpireSeconds"
          :options="expireOptions"
          style="width: 100%"
        />
      </n-form-item>
      <n-text depth="3" style="font-size: 12px; display: block; margin-bottom: 16px">
        {{ t('settings.jwtExpireHint') }}
      </n-text>
      <n-button type="primary" :loading="saving" @click="handleSave">
        {{ t('common.save') }}
      </n-button>
    </n-form>
  </n-spin>
</template>
