<script setup lang="ts">
import { ref, inject, onMounted, watch } from 'vue'
import type { Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { authApi } from '@/api'
import { GlobeOutline, SunnyOutline, MoonOutline, LogInOutline } from '@vicons/ionicons5'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const authStore = useAuthStore()
const { t, locale } = useI18n()

const isDark = inject<Ref<boolean>>('isDark', ref(true))
const toggleTheme = inject<() => void>('toggleTheme', () => {})

const form = ref({
  username: '',
  password: '',
})
const loading = ref(false)
const loginError = ref('')

// OIDC SSO state
const oidcEnabled = ref(false)
const oidcLoginUrl = ref('')
const oidcLoading = ref(false)

const langOptions = [
  { label: '简体中文', value: 'zh-CN' },
  { label: 'English', value: 'en' },
]

function handleLangChange(val: string) {
  locale.value = val
  localStorage.setItem('locale', val)
}

async function handleLogin() {
  loginError.value = ''
  if (!form.value.username || !form.value.password) {
    loginError.value = t('auth.pleaseEnter') || 'Please enter username and password'
    return
  }

  loading.value = true
  try {
    await authStore.login(form.value.username, form.value.password)
    message.success(t('auth.loginSuccess'))
    router.push((route.query.redirect as string) || '/dashboard')
  } catch (err: any) {
    loginError.value = err.message || t('auth.loginFailed')
  } finally {
    loading.value = false
  }
}

function handleSSOLogin() {
  if (oidcLoginUrl.value) {
    window.location.href = oidcLoginUrl.value
  }
}

async function checkOIDCConfig() {
  try {
    const { data } = await authApi.getOIDCConfig()
    if (data.data.enabled && data.data.login_url) {
      oidcEnabled.value = true
      oidcLoginUrl.value = data.data.login_url
    }
  } catch {
    // OIDC not configured, that's fine
  }
}

onMounted(() => {
  checkOIDCConfig()
})

watch([() => form.value.username, () => form.value.password], () => {
  if (loginError.value) loginError.value = ''
})
</script>

<template>
  <div class="login-container" :class="{ light: !isDark }">
    <!-- Aurora is already rendered globally in App.vue (fixed, z-index: -2).
         Only add the grid texture layer here. -->
    <div class="grid-lines" :class="{ light: !isDark }" />

    <!-- Top right controls: language + theme -->
    <div class="login-controls">
      <n-select
        :value="locale"
        :options="langOptions"
        size="small"
        style="width: 120px"
        @update:value="handleLangChange"
      />
      <n-button text @click="toggleTheme" style="padding: 4px 8px">
        <n-icon :component="isDark ? SunnyOutline : MoonOutline" :size="18" />
      </n-button>
    </div>

    <div class="login-card conic-border noise-overlay" :class="{ light: !isDark }">
      <div class="login-header">
        <img src="/logo.svg" alt="SREAgent" class="login-logo" />
        <h1 class="logo-text">
          <span class="gradient-text">SRE</span><span class="agent-text" :class="{ light: !isDark }">Agent</span>
        </h1>
        <p class="eyebrow" style="margin-top:10px;margin-bottom:0">SRE · Alert Intelligence</p>
        <p class="login-subtitle" :class="{ light: !isDark }">{{ t('auth.subtitle') }}</p>
      </div>

      <n-form @submit.prevent="handleLogin">
        <n-form-item :label="t('auth.username')" :show-feedback="false" style="margin-bottom: 20px">
          <n-input
            v-model:value="form.username"
            :placeholder="t('auth.enterUsername') || 'Enter username'"
            size="large"
            :autofocus="true"
          />
        </n-form-item>

        <n-form-item :label="t('auth.password')" :show-feedback="false" style="margin-bottom: 28px">
          <n-input
            v-model:value="form.password"
            type="password"
            :placeholder="t('auth.enterPassword') || 'Enter password'"
            size="large"
            show-password-on="click"
            @keyup.enter="handleLogin"
          />
        </n-form-item>

        <n-button
          type="primary"
          block
          size="large"
          :loading="loading"
          @click="handleLogin"
          style="height: 44px; font-size: 16px"
        >
          {{ t('auth.signIn') }}
        </n-button>
      </n-form>

      <!-- Inline login error -->
      <div v-if="loginError" class="login-error">
        <n-alert type="error" :show-icon="true" :closable="false" style="margin-top: 16px">
          {{ loginError }}
        </n-alert>
      </div>

      <!-- SSO Login -->
      <div v-if="oidcEnabled" class="sso-section">
        <n-divider>
          <n-text depth="3" style="font-size: 12px">{{ t('auth.orContinueWith') }}</n-text>
        </n-divider>
        <n-button
          block
          size="large"
          secondary
          @click="handleSSOLogin"
          :loading="oidcLoading"
          style="height: 44px; font-size: 14px"
        >
          <template #icon><n-icon :component="LogInOutline" /></template>
          {{ t('auth.ssoLogin') }}
        </n-button>
      </div>

    </div>
  </div>
</template>

<style scoped>
.login-container {
  min-height: 100vh;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  /* Solid fallback so the page is never invisible */
  background: var(--sre-bg-base, #07090d);
  overflow: hidden;
  transition: background var(--sre-duration-slow) var(--sre-ease-out);
}

.login-container.light {
  background: var(--sre-bg-page, #f3f5f8);
}

.login-controls {
  position: absolute;
  top: 20px;
  right: 24px;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Grid lines — subtle depth layer */
.grid-lines {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.025) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.025) 1px, transparent 1px);
  background-size: 60px 60px;
}
.grid-lines.light {
  background-image:
    linear-gradient(rgba(0, 0, 0, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(0, 0, 0, 0.03) 1px, transparent 1px);
}

/* Login card — glass + conic border via utility classes */
.login-card {
  width: 420px;
  padding: 52px 44px;
  border-radius: var(--sre-radius-2xl);
  position: relative;
  z-index: 2;
  /* glass base overridden by .surface-glass-strong */
  background: color-mix(in srgb, var(--sre-bg-card) 62%, transparent);
  backdrop-filter: saturate(170%) blur(24px);
  -webkit-backdrop-filter: saturate(170%) blur(24px);
  border: 1px solid var(--sre-border-strong);
  box-shadow: var(--sre-shadow-soft-xl);
  animation: sre-scale-in var(--sre-duration-slow) var(--sre-ease-spring) both;
}

.login-card.light {
  background: color-mix(in srgb, #ffffff 82%, transparent);
  border-color: rgba(0,0,0,0.08);
}

.login-header {
  text-align: center;
  margin-bottom: 36px;
}

.login-logo {
  width: 60px;
  height: 60px;
  display: block;
  margin: 0 auto 16px;
  filter: drop-shadow(0 8px 28px rgba(24, 160, 88, 0.50));
  animation: sre-bounce-in 0.6s var(--sre-ease-spring) 0.1s both;
}

.logo-text {
  font-size: 38px;
  font-weight: 700;
  margin: 0 0 6px 0;
  letter-spacing: -1.5px;
}

.agent-text {
  color: var(--sre-text-primary);
  font-weight: 300;
  transition: color var(--sre-duration-slow) var(--sre-ease-out);
}

.agent-text.light {
  color: rgba(15,23,42,0.85);
}

.login-subtitle {
  color: var(--sre-text-tertiary);
  font-size: var(--sre-fs-sm);
  margin: 6px 0 0;
  transition: color var(--sre-duration-slow) var(--sre-ease-out);
}
.login-subtitle.light { color: rgba(0, 0, 0, 0.42); }

.sso-section { margin-top: 16px; }

</style>
