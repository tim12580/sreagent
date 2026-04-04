<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { oidcSettingsApi } from '@/api'

const message = useMessage()
const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)

const form = reactive({
  enabled: false,
  issuer_url: '',
  client_id: '',
  client_secret: '',
  redirect_url: '',
  scopes: 'openid,profile,email',
  role_claim: 'realm_access.roles',
  role_mapping: '',
  default_role: 'viewer',
  auto_provision: true,
})

const defaultRoleOptions = [
  { label: 'admin', value: 'admin' },
  { label: 'team_lead', value: 'team_lead' },
  { label: 'member', value: 'member' },
  { label: 'viewer', value: 'viewer' },
]

async function fetchConfig() {
  loading.value = true
  try {
    const res = await oidcSettingsApi.getConfig()
    if (res.data.data) {
      const d = res.data.data
      form.enabled = d.enabled
      form.issuer_url = d.issuer_url || ''
      form.client_id = d.client_id || ''
      form.client_secret = d.client_secret || ''
      form.redirect_url = d.redirect_url || ''
      form.scopes = d.scopes || 'openid,profile,email'
      form.role_claim = d.role_claim || 'realm_access.roles'
      form.role_mapping = d.role_mapping || ''
      form.default_role = d.default_role || 'viewer'
      form.auto_provision = d.auto_provision
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
    await oidcSettingsApi.updateConfig({ ...form })
    message.success(t('common.savedSuccess'))
  } catch (err: any) {
    message.error(err.message)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  fetchConfig()
})
</script>

<template>
  <n-spin :show="loading">
    <div style="max-width: 640px; margin: 0 auto; padding: 24px 0">
      <!-- Restart warning banner -->
      <n-alert type="warning" :show-icon="true" style="margin-bottom: 20px">
        {{ t('settings.oidcRestartWarning') }}
      </n-alert>

      <n-form label-placement="top">
        <n-form-item :label="t('settings.oidcEnabled')">
          <n-switch v-model:value="form.enabled" />
        </n-form-item>

        <n-form-item :label="t('settings.oidcIssuerUrl')">
          <n-input
            v-model:value="form.issuer_url"
            :placeholder="t('settings.oidcIssuerUrlPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcClientId')">
          <n-input
            v-model:value="form.client_id"
            :placeholder="t('settings.oidcClientIdPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcClientSecret')">
          <n-input
            v-model:value="form.client_secret"
            type="password"
            show-password-on="click"
            :placeholder="t('settings.oidcClientSecretPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcRedirectUrl')">
          <n-input
            v-model:value="form.redirect_url"
            :placeholder="t('settings.oidcRedirectUrlPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcScopes')">
          <n-input
            v-model:value="form.scopes"
            :placeholder="t('settings.oidcScopesPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcRoleClaim')">
          <n-input
            v-model:value="form.role_claim"
            :placeholder="t('settings.oidcRoleClaimPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcRoleMapping')">
          <n-input
            v-model:value="form.role_mapping"
            type="textarea"
            :rows="3"
            :placeholder="t('settings.oidcRoleMappingPlaceholder')"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcDefaultRole')">
          <n-select
            v-model:value="form.default_role"
            :options="defaultRoleOptions"
            style="width: 100%"
          />
        </n-form-item>

        <n-form-item :label="t('settings.oidcAutoProvision')">
          <n-switch v-model:value="form.auto_provision" />
          <span style="margin-left: 8px; color: #888; font-size: 13px">
            {{ t('settings.oidcAutoProvisionHint') }}
          </span>
        </n-form-item>

        <n-button type="primary" :loading="saving" @click="saveConfig">
          {{ t('common.save') }}
        </n-button>
      </n-form>
    </div>
  </n-spin>
</template>
