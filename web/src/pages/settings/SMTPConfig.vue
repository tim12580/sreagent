<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { smtpSettingsApi } from '@/api'

const message = useMessage()
const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const testTo = ref('')

const form = reactive({
  enabled: false,
  smtp_host: '',
  smtp_port: 587,
  smtp_tls: true,
  username: '',
  password: '',
  from: '',
})

async function fetchConfig() {
  loading.value = true
  try {
    const res = await smtpSettingsApi.getConfig()
    if (res.data.data) {
      const d = res.data.data
      form.enabled = d.enabled
      form.smtp_host = d.smtp_host || ''
      form.smtp_port = d.smtp_port || 587
      form.smtp_tls = d.smtp_tls ?? true
      form.username = d.username || ''
      form.password = d.password || ''
      form.from = d.from || ''
    }
  } catch (err: any) {
    message.error(err.message)
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  saving.value = true
  try {
    await smtpSettingsApi.updateConfig({ ...form })
    message.success(t('common.success'))
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

async function testConnection() {
  if (!testTo.value) {
    message.warning(t('smtp.enterTestEmail'))
    return
  }
  testing.value = true
  try {
    const res = await smtpSettingsApi.testConnection(testTo.value)
    message.success(res.data.data?.message || t('common.success'))
  } catch (err: any) {
    message.error(err.message)
  } finally {
    testing.value = false
  }
}

onMounted(fetchConfig)
</script>

<template>
  <div class="smtp-config">
    <n-spin :show="loading">
      <n-form :model="form" label-placement="left" label-width="160">
        <n-form-item :label="t('smtp.enabled')">
          <n-switch v-model:value="form.enabled" />
        </n-form-item>

        <n-divider>{{ t('smtp.serverSettings') }}</n-divider>

        <n-grid :cols="2" :x-gap="16">
          <n-gi>
            <n-form-item :label="t('smtp.host')">
              <n-input v-model:value="form.smtp_host" :placeholder="t('smtp.hostPlaceholder')" />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="t('smtp.port')">
              <n-input-number v-model:value="form.smtp_port" :min="1" :max="65535" style="width: 100%" />
            </n-form-item>
          </n-gi>
        </n-grid>

        <n-form-item :label="t('smtp.tls')">
          <n-switch v-model:value="form.smtp_tls" />
          <span style="margin-left: 8px; color: var(--n-text-color-3); font-size: 12px;">
            {{ t('smtp.tlsHint') }}
          </span>
        </n-form-item>

        <n-divider>{{ t('smtp.credentials') }}</n-divider>

        <n-form-item :label="t('smtp.username')">
          <n-input v-model:value="form.username" :placeholder="t('smtp.usernamePlaceholder')" />
        </n-form-item>

        <n-form-item :label="t('smtp.password')">
          <n-input
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :placeholder="form.password === '********' ? t('smtp.passwordMasked') : t('smtp.passwordPlaceholder')"
            @focus="if (form.password === '********') form.password = ''"
          />
        </n-form-item>

        <n-form-item :label="t('smtp.from')">
          <n-input v-model:value="form.from" :placeholder="t('smtp.fromPlaceholder')" />
        </n-form-item>

        <n-form-item>
          <n-space>
            <n-button type="primary" :loading="saving" @click="saveConfig">
              {{ t('common.save') }}
            </n-button>
          </n-space>
        </n-form-item>

        <n-divider>{{ t('smtp.testSection') }}</n-divider>

        <n-form-item :label="t('smtp.testRecipient')">
          <n-input v-model:value="testTo" :placeholder="t('smtp.testRecipientPlaceholder')" style="max-width: 320px" />
        </n-form-item>
        <n-form-item>
          <n-button :loading="testing" @click="testConnection">
            {{ t('smtp.sendTest') }}
          </n-button>
        </n-form-item>
      </n-form>
    </n-spin>
  </div>
</template>

<style scoped>
.smtp-config {
  max-width: 800px;
}
</style>
