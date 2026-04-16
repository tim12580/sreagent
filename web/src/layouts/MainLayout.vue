<script setup lang="ts">
import { ref, computed, watch, h, inject, onMounted, onUnmounted } from 'vue'
import type { Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NIcon, useMessage } from 'naive-ui'
import type { MenuOption, DropdownOption } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { userNotifyConfigApi, authApi } from '@/api'
import type { UserNotifyConfig } from '@/types'
import {
  GridOutline,
  ServerOutline,
  AlertCircleOutline,
  CalendarOutline,
  SettingsOutline,
  LogOutOutline,
  NotificationsOutline,
  SunnyOutline,
  MoonOutline,
  ChevronDownOutline,
  PersonOutline,
  LockClosedOutline,
  EarthOutline,
  TimeOutline,
} from '@vicons/ionicons5'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t, locale } = useI18n()
const message = useMessage()

const collapsed = ref(false)

const isDark = inject<Ref<boolean>>('isDark', ref(true))
const toggleTheme = inject<() => void>('toggleTheme', () => {})

onMounted(() => {
  if (authStore.isLoggedIn && !authStore.user) {
    authStore.fetchProfile()
  }
})

// ===== Clock =====
const timeDisplay = ref('')   // HH:mm:ss
const dateDisplay = ref('')   // YYYY-MM-DD
const timezone = ref(localStorage.getItem('sre-timezone') || 'Asia/Shanghai')
const showTzPanel = ref(false)

const timezoneOptions = [
  { label: 'Asia/Shanghai', abbr: 'CST', value: 'Asia/Shanghai' },
  { label: 'UTC',           abbr: 'UTC', value: 'UTC' },
  { label: 'Asia/Tokyo',    abbr: 'JST', value: 'Asia/Tokyo' },
  { label: 'Asia/Singapore',abbr: 'SGT', value: 'Asia/Singapore' },
  { label: 'Europe/London', abbr: 'GMT', value: 'Europe/London' },
  { label: 'America/New_York', abbr: 'EST', value: 'America/New_York' },
  { label: 'America/Los_Angeles', abbr: 'PST', value: 'America/Los_Angeles' },
]

const tzAbbr = computed(() => {
  return timezoneOptions.find(o => o.value === timezone.value)?.abbr || timezone.value.split('/').pop() || 'TZ'
})

function updateClock() {
  const now = new Date()
  timeDisplay.value = now.toLocaleTimeString('en-GB', {
    timeZone: timezone.value,
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  })
  dateDisplay.value = now.toLocaleDateString(locale.value === 'zh-CN' ? 'zh-CN' : 'en-US', {
    timeZone: timezone.value,
    year: 'numeric', month: 'short', day: '2-digit',
  })
}

let clockInterval: ReturnType<typeof setInterval>
onMounted(() => { updateClock(); clockInterval = setInterval(updateClock, 1000) })
onUnmounted(() => clearInterval(clockInterval))

function selectTimezone(val: string) {
  timezone.value = val
  localStorage.setItem('sre-timezone', val)
  showTzPanel.value = false
  updateClock()
}

// ===== Menu =====
function renderIcon(icon: any) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

const menuOptions = computed<MenuOption[]>(() => {
  const items: MenuOption[] = [
    { label: t('menu.dashboard'),        key: '/dashboard',  icon: renderIcon(GridOutline) },
    { label: t('menu.datasources'),      key: '/datasources', icon: renderIcon(ServerOutline) },
    {
      label: t('menu.alertManagement'),  key: '/alerts', icon: renderIcon(AlertCircleOutline),
      children: [
        { label: t('menu.alertRules'),   key: '/alerts/rules' },
        { label: t('menu.activeAlerts'), key: '/alerts/events' },
        { label: t('menu.alertHistory'), key: '/alerts/history' },
        { label: t('menu.muteRules'),    key: '/alerts/mute-rules' },
      ],
    },
    { label: t('menu.notification'), key: '/notification', icon: renderIcon(NotificationsOutline) },
    { label: t('menu.schedule'), key: '/schedule',  icon: renderIcon(CalendarOutline) },
  ]
  // Settings page is only visible to admin and team_lead roles
  if (authStore.canManage) {
    items.push({ label: t('menu.settings'), key: '/settings',  icon: renderIcon(SettingsOutline) })
  }
  return items
})

function resolveActiveKey(p: string): string {
  if (p.startsWith('/alerts/rules'))      return '/alerts/rules'
  if (p.startsWith('/alerts/events'))     return '/alerts/events'
  if (p.startsWith('/alerts/history'))    return '/alerts/history'
  if (p.startsWith('/alerts/mute-rules')) return '/alerts/mute-rules'
  if (p.startsWith('/notification'))      return '/notification'
  return p
}

// menuSelectedKey is driven by the route but can be briefly cleared so that
// Naive UI's n-menu always fires @update:value (it suppresses the event when
// the clicked key equals the current :value — this causes the "clicking
// Settings does nothing" bug when the user is already on /settings).
const menuSelectedKey = ref(resolveActiveKey(route.path))
watch(
  () => route.path,
  (p) => { menuSelectedKey.value = resolveActiveKey(p) },
)

function handleMenuClick(key: string) {
  // Temporarily clear the selected key so n-menu will fire @update:value
  // even if the user clicks the item that is already active.
  menuSelectedKey.value = ''
  router.push(key)
}

// ===== Language =====
const langOptions = [
  { label: '简体中文', value: 'zh-CN' },
  { label: 'English',  value: 'en' },
]
function handleLangChange(val: string) { locale.value = val; localStorage.setItem('locale', val) }

// ===== User =====
const userDropdownOptions = computed<DropdownOption[]>(() => [
  { label: t('header.profile'),        key: 'profile',  icon: renderIcon(PersonOutline) },
  { label: t('header.changePassword'), key: 'password', icon: renderIcon(LockClosedOutline) },
  { type: 'divider', key: 'd1' },
  { label: t('header.logout'),         key: 'logout',   icon: renderIcon(LogOutOutline) },
])

async function handleUserDropdown(key: string) {
  if (key === 'logout') {
    authStore.logout()
    router.push('/login')
  } else if (key === 'profile') {
    profileTab.value = 'info'
    await openProfileModal()
  } else if (key === 'password') {
    profileTab.value = 'password'
    await openProfileModal()
  }
}

const userInitial  = computed(() => (authStore.user?.display_name || authStore.user?.username || 'U').charAt(0).toUpperCase())
const displayName  = computed(() => authStore.user?.display_name || authStore.user?.username || 'User')

// ===== Breadcrumb =====
const pageTitle = computed(() => {
  const p = route.path
  if (p === '/dashboard')                         return t('menu.dashboard')
  if (p === '/datasources')                       return t('menu.datasources')
  if (p.startsWith('/alerts/rules'))              return t('menu.alertRules')
  if (p.startsWith('/alerts/events'))             return t('menu.activeAlerts')
  if (p.startsWith('/alerts/history'))            return t('menu.alertHistory')
  if (p.startsWith('/alerts/mute-rules'))         return t('menu.muteRules')
  if (p.startsWith('/notification'))              return t('menu.notification')
  if (p === '/schedule')                          return t('menu.schedule')
  if (p === '/settings')                          return t('menu.settings')
  return ''
})
const parentTitle = computed(() => {
  const p = route.path
  if (p.startsWith('/alerts/'))        return t('menu.alertManagement')
  if (p.startsWith('/notification'))   return ''
  return ''
})

// ===== Profile Modal =====
const showProfileModal = ref(false)
const profileTab = ref('info')  // 'info' | 'password' | 'notify'
const profileSaving = ref(false)

// Tab: Basic info
const profileForm = ref({ display_name: '', email: '', phone: '', avatar: '' })

// Preset avatars (emoji-based, no upload server needed for MVP)
const presetAvatars = ['👤','🧑‍💻','👩‍💻','🧑‍🔧','👩‍🔧','🧑‍🚀','👩‍🚀','🦊','🐺','🐧','🦅','🦁']

// Tab: Change password
const pwdForm = ref({ old_password: '', new_password: '', confirm_password: '' })
const pwdError = ref('')

// Tab: Notify config (multi-config list)
const userNotifyConfigs = ref<UserNotifyConfig[]>([])
const newNotifyConfig = ref<{ media_type: 'lark_personal' | 'email' | 'webhook'; config: string }>({ media_type: 'lark_personal', config: '' })

const mediaTypeOptions = computed(() => [
  { label: t('profile.larkPersonal'), value: 'lark_personal' },
  { label: t('profile.email'),        value: 'email' },
  { label: t('profile.webhook'),      value: 'webhook' },
])
const configHint = computed(() => {
  switch (newNotifyConfig.value.media_type) {
    case 'lark_personal': return t('profile.larkUserIdHint')
    case 'email':         return t('profile.emailHint')
    case 'webhook':       return t('profile.webhookHint')
    default:              return ''
  }
})

async function openProfileModal() {
  // 注意：不在此处重置 profileTab，由调用方决定打开哪个 tab
  profileSaving.value = false
  pwdError.value = ''
  pwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
  // Pre-fill from store
  profileForm.value = {
    display_name: authStore.user?.display_name || '',
    email:        authStore.user?.email || '',
    phone:        authStore.user?.phone || '',
    avatar:       authStore.user?.avatar || '',
  }
  // Load notify configs list
  try {
    const cfgs = await userNotifyConfigApi.list()
    userNotifyConfigs.value = cfgs.data.data || []
  } catch {
    userNotifyConfigs.value = []
  }
  newNotifyConfig.value = { media_type: 'lark_personal', config: '' }
  showProfileModal.value = true
}

async function saveProfile() {
  profileSaving.value = true
  try {
    await authApi.updateMe(profileForm.value)
    await authStore.fetchProfile()
    message.success(t('profile.saved'))
  } catch (err: any) {
    message.error(err.message || t('common.failed'))
  } finally {
    profileSaving.value = false
  }
}

async function savePassword() {
  pwdError.value = ''
  if (pwdForm.value.new_password !== pwdForm.value.confirm_password) {
    pwdError.value = t('profile.passwordMismatch')
    return
  }
  profileSaving.value = true
  try {
    await authApi.changeMyPassword({ old_password: pwdForm.value.old_password, new_password: pwdForm.value.new_password })
    message.success(t('profile.passwordChanged'))
    pwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
  } catch (err: any) {
    message.error(err.message || t('common.failed'))
  } finally {
    profileSaving.value = false
  }
}

async function addNotifyConfig() {
  if (!newNotifyConfig.value.config) return
  profileSaving.value = true
  try {
    await userNotifyConfigApi.upsert({ ...newNotifyConfig.value, is_enabled: true })
    // Reload list
    const cfgs = await userNotifyConfigApi.list()
    userNotifyConfigs.value = cfgs.data.data || []
    newNotifyConfig.value = { media_type: 'lark_personal', config: '' }
    message.success(t('profile.notifyConfigSaved'))
  } catch (err: any) {
    message.error(err.message || t('common.failed'))
  } finally {
    profileSaving.value = false
  }
}

async function removeNotifyConfig(mediaType: string) {
  try {
    await userNotifyConfigApi.deleteByType(mediaType)
    userNotifyConfigs.value = userNotifyConfigs.value.filter(c => c.media_type !== mediaType)
  } catch (err: any) {
    message.error(err.message || t('common.failed'))
  }
}

async function toggleNotifyConfig(cfg: UserNotifyConfig, enabled: boolean) {
  try {
    await userNotifyConfigApi.upsert({ ...cfg, is_enabled: enabled })
  } catch (err: any) {
    message.error(err.message || t('common.failed'))
  }
}
</script>

<template>
  <n-layout has-sider style="height: 100vh">

    <!-- ===== Sidebar ===== -->
    <n-layout-sider
      bordered
      collapse-mode="width"
      :collapsed-width="64"
      :width="240"
      :collapsed="collapsed"
      show-trigger
      @collapse="collapsed = true"
      @expand="collapsed = false"
      :native-scrollbar="false"
      style="background: var(--sre-bg-card)"
    >
      <div class="sider-logo" :class="{ collapsed }">
        <div class="logo-mark">S</div>
        <transition name="fade">
          <span v-if="!collapsed" class="logo-text">
            <span class="gradient-text">SRE</span>Agent
          </span>
        </transition>
      </div>

      <n-menu
        :collapsed="collapsed"
        :collapsed-width="64"
        :collapsed-icon-size="22"
        :options="menuOptions"
        :value="menuSelectedKey"
        @update:value="handleMenuClick"
      />
    </n-layout-sider>

    <!-- ===== Right: header + content ===== -->
    <n-layout>

      <!-- Header Bar -->
      <div class="header-bar">

        <!-- Left: breadcrumb -->
        <div class="header-left">
          <n-breadcrumb v-if="parentTitle" separator=">">
            <n-breadcrumb-item>{{ parentTitle }}</n-breadcrumb-item>
            <n-breadcrumb-item>{{ pageTitle }}</n-breadcrumb-item>
          </n-breadcrumb>
          <span v-else class="header-page-title">{{ pageTitle }}</span>
        </div>

        <!-- Right: clock + controls -->
        <div class="header-right">

          <!-- ① Time+Timezone pill (integrated component) -->
          <n-popover
            v-model:show="showTzPanel"
            trigger="click"
            placement="bottom-end"
            :show-arrow="false"
            style="padding: 0"
          >
            <template #trigger>
              <div class="clock-pill" :class="{ active: showTzPanel }">
                <n-icon :component="TimeOutline" :size="13" class="clock-icon" />
                <span class="clock-time">{{ timeDisplay }}</span>
                <span class="clock-sep">·</span>
                <span class="clock-date">{{ dateDisplay }}</span>
                <span class="clock-tz">{{ tzAbbr }}</span>
              </div>
            </template>

            <!-- Timezone picker dropdown -->
            <div class="tz-panel">
              <div class="tz-panel-title">
                <n-icon :component="EarthOutline" :size="14" />
                {{ t('header.timezone') }}
              </div>
              <div
                v-for="opt in timezoneOptions"
                :key="opt.value"
                class="tz-option"
                :class="{ selected: timezone === opt.value }"
                @click="selectTimezone(opt.value)"
              >
                <span class="tz-opt-abbr">{{ opt.abbr }}</span>
                <span class="tz-opt-label">{{ opt.label }}</span>
                <span v-if="timezone === opt.value" class="tz-opt-check">✓</span>
              </div>
            </div>
          </n-popover>

          <div class="header-sep" />

          <!-- ② Language — icon + text, no visible border -->
          <n-popselect
            :value="locale"
            :options="langOptions"
            trigger="click"
            :render-label="(opt: any) => opt.label"
            @update:value="handleLangChange"
          >
            <div class="ctrl-btn">
              <n-icon :component="EarthOutline" :size="15" />
              <span class="ctrl-label">{{ locale === 'zh-CN' ? '中' : 'EN' }}</span>
            </div>
          </n-popselect>

          <!-- ③ Theme toggle -->
          <div class="ctrl-btn" @click="toggleTheme" :title="isDark ? t('header.lightMode') : t('header.darkMode')">
            <n-icon :component="isDark ? SunnyOutline : MoonOutline" :size="16" />
          </div>

          <div class="header-sep" />

          <!-- ④ User -->
          <n-dropdown :options="userDropdownOptions" trigger="click" @select="handleUserDropdown">
            <div class="user-pill">
              <div class="user-avatar">{{ userInitial }}</div>
              <span class="user-name">{{ displayName }}</span>
              <n-icon :component="ChevronDownOutline" :size="12" class="user-chevron" />
            </div>
          </n-dropdown>

        </div>
      </div>

      <!-- Main content -->
      <n-layout-content :native-scrollbar="false" style="padding: 24px; background: var(--sre-bg-page)">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-layout>

  <!-- ===== Profile Modal ===== -->
  <n-modal
    v-model:show="showProfileModal"
    :title="t('profile.title')"
    preset="card"
    style="width: 500px"
    :bordered="false"
    :segmented="{ content: true }"
  >
    <n-tabs v-model:value="profileTab" type="line" animated>

      <!-- Tab 1: Basic info -->
      <n-tab-pane name="info" :tab="t('profile.tabInfo')">
        <!-- Avatar selector -->
        <div class="avatar-section">
          <div class="avatar-current">
            <span class="avatar-preview">{{ profileForm.avatar || userInitial }}</span>
          </div>
          <div class="avatar-grid">
            <span
              v-for="a in presetAvatars"
              :key="a"
              class="avatar-option"
              :class="{ selected: profileForm.avatar === a }"
              @click="profileForm.avatar = a"
            >{{ a }}</span>
          </div>
        </div>

        <n-form label-placement="top" size="small" style="margin-top: 16px">
          <n-form-item :label="t('auth.username')">
            <n-input :value="authStore.user?.username" disabled />
          </n-form-item>
          <n-form-item :label="t('settings.displayName')">
            <n-input v-model:value="profileForm.display_name" :placeholder="t('settings.displayName')" />
          </n-form-item>
          <n-form-item :label="t('settings.email')">
            <n-input v-model:value="profileForm.email" :placeholder="t('settings.email')" />
          </n-form-item>
          <n-form-item :label="t('settings.phone') || '手机号'">
            <n-input v-model:value="profileForm.phone" placeholder="+86 138..." />
          </n-form-item>
        </n-form>

        <div class="modal-footer">
          <n-button type="primary" :loading="profileSaving" @click="saveProfile">{{ t('common.save') }}</n-button>
        </div>
      </n-tab-pane>

      <!-- Tab 2: Change password -->
      <n-tab-pane name="password" :tab="t('profile.tabPassword')">
        <n-form label-placement="top" size="small">
          <n-form-item :label="t('profile.oldPassword')">
            <n-input v-model:value="pwdForm.old_password" type="password" show-password-on="click" />
          </n-form-item>
          <n-form-item :label="t('profile.newPassword')">
            <n-input v-model:value="pwdForm.new_password" type="password" show-password-on="click" />
          </n-form-item>
          <n-form-item :label="t('profile.confirmPassword')">
            <n-input
              v-model:value="pwdForm.confirm_password"
              type="password"
              show-password-on="click"
              :status="pwdError ? 'error' : undefined"
            />
            <template #feedback>
              <span v-if="pwdError" style="color: var(--sre-danger)">{{ pwdError }}</span>
            </template>
          </n-form-item>
        </n-form>

        <div class="modal-footer">
          <n-button type="primary" :loading="profileSaving" @click="savePassword">{{ t('profile.changePassword') }}</n-button>
        </div>
      </n-tab-pane>

      <!-- Tab 3: Multi-notify config -->
      <n-tab-pane name="notify" :tab="t('profile.tabNotify')">
        <!-- List of existing configs -->
        <div class="notify-config-list">
          <div v-for="cfg in userNotifyConfigs" :key="cfg.media_type" class="notify-config-item">
            <div class="notify-config-info">
              <n-tag size="small" :type="cfg.media_type === 'lark_personal' ? 'success' : cfg.media_type === 'email' ? 'info' : 'default'">
                {{ cfg.media_type === 'lark_personal' ? t('profile.larkPersonal') : cfg.media_type === 'email' ? t('profile.email') : t('profile.webhook') }}
              </n-tag>
              <span class="notify-config-value">{{ cfg.config }}</span>
              <n-switch v-model:value="cfg.is_enabled" size="small" @update:value="(v: boolean) => toggleNotifyConfig(cfg, v)" />
            </div>
            <n-button size="tiny" quaternary type="error" @click="removeNotifyConfig(cfg.media_type)">{{ t('common.remove') }}</n-button>
          </div>
          <n-empty v-if="userNotifyConfigs.length === 0" :description="t('profile.noNotifyConfig')" style="padding: 20px 0" />
        </div>

        <!-- Add new config -->
        <n-divider>{{ t('profile.addNotify') }}</n-divider>
        <n-form label-placement="top" size="small">
          <n-form-item :label="t('profile.mediaType')">
            <n-select v-model:value="newNotifyConfig.media_type" :options="mediaTypeOptions" />
          </n-form-item>
          <n-form-item :label="t('profile.configValue')">
            <n-input v-model:value="newNotifyConfig.config" :placeholder="configHint" clearable />
          </n-form-item>
        </n-form>
        <div class="modal-footer">
          <n-button type="primary" :loading="profileSaving" @click="addNotifyConfig">{{ t('profile.addNotify') }}</n-button>
        </div>
      </n-tab-pane>

    </n-tabs>
  </n-modal>
</template>

<style scoped>
/* ===== Sidebar ===== */
.sider-logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 20px;
  height: 52px;
  border-bottom: 1px solid var(--sre-border);
  transition: all 0.3s;
}
.sider-logo.collapsed {
  justify-content: center;
  padding: 16px;
}
.logo-mark {
  width: 28px;
  height: 28px;
  border-radius: 7px;
  background: linear-gradient(135deg, #18a058, #36ad6a);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 15px;
  color: white;
  flex-shrink: 0;
  box-shadow: 0 2px 8px rgba(24, 160, 88, 0.35);
}
.logo-text {
  font-size: 18px;
  font-weight: 600;
  white-space: nowrap;
  color: var(--sre-text-primary);
}

/* ===== Header ===== */
.header-bar {
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  border-bottom: 1px solid var(--sre-border);
  background: var(--sre-bg-card);
  flex-shrink: 0;
  transition: background 0.3s, border-color 0.3s;
}
.header-left {
  display: flex;
  align-items: center;
}
.header-page-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--sre-text-primary);
  opacity: 0.85;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

/* Subtle separator */
.header-sep {
  width: 1px;
  height: 16px;
  background: var(--sre-border);
  margin: 0 6px;
  opacity: 0.6;
}

/* ===== Clock Pill ===== */
.clock-pill {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 8px;
  cursor: pointer;
  user-select: none;
  transition: background 0.2s, box-shadow 0.2s;
  border: 1px solid transparent;
}
.clock-pill:hover,
.clock-pill.active {
  background: rgba(24, 160, 88, 0.06);
  border-color: rgba(24, 160, 88, 0.18);
}
.clock-icon {
  color: var(--sre-primary);
  flex-shrink: 0;
  opacity: 0.7;
}
.clock-time {
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  font-weight: 600;
  color: var(--sre-text-primary);
  letter-spacing: 0.8px;
  /* subtle digit-flip animation */
  transition: opacity 0.1s;
}
.clock-sep {
  color: var(--sre-text-secondary);
  font-size: 11px;
}
.clock-date {
  font-size: 11px;
  color: var(--sre-text-secondary);
  letter-spacing: 0.2px;
}
.clock-tz {
  font-size: 10px;
  font-weight: 600;
  color: var(--sre-primary);
  background: rgba(24, 160, 88, 0.1);
  padding: 1px 5px;
  border-radius: 4px;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

/* ===== Timezone panel ===== */
.tz-panel {
  min-width: 220px;
  padding: 6px 0;
}
.tz-panel-title {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--sre-text-secondary);
  letter-spacing: 0.5px;
  text-transform: uppercase;
  border-bottom: 1px solid var(--sre-border);
  margin-bottom: 4px;
}
.tz-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s;
  color: var(--sre-text-primary);
}
.tz-option:hover { background: rgba(128, 128, 128, 0.08); }
.tz-option.selected { color: var(--sre-primary); }
.tz-opt-abbr {
  font-weight: 700;
  font-size: 11px;
  width: 32px;
  color: var(--sre-primary);
  flex-shrink: 0;
}
.tz-opt-label { flex: 1; }
.tz-opt-check {
  font-size: 12px;
  color: var(--sre-primary);
  font-weight: 700;
}

/* ===== Control buttons (lang / theme) ===== */
.ctrl-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 5px 8px;
  border-radius: 7px;
  cursor: pointer;
  color: var(--sre-text-secondary);
  transition: background 0.2s, color 0.2s;
  font-size: 13px;
  font-weight: 500;
}
.ctrl-btn:hover {
  background: rgba(128, 128, 128, 0.1);
  color: var(--sre-text-primary);
}
.ctrl-label {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

/* ===== User pill ===== */
.user-pill {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 4px 10px 4px 4px;
  border-radius: 9px;
  cursor: pointer;
  transition: background 0.2s;
  border: 1px solid transparent;
}
.user-pill:hover {
  background: rgba(128, 128, 128, 0.08);
  border-color: var(--sre-border);
}
.user-avatar {
  width: 26px;
  height: 26px;
  border-radius: 7px;
  background: linear-gradient(135deg, #18a058, #36ad6a);
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 1px 4px rgba(24, 160, 88, 0.3);
}
.user-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--sre-text-primary);
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.user-chevron {
  color: var(--sre-text-secondary);
  flex-shrink: 0;
}

/* ===== Transitions ===== */
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* ===== Profile Modal ===== */
.avatar-section { display: flex; align-items: center; gap: 16px; padding: 12px 0 4px; }
.avatar-current {
  width: 52px; height: 52px; border-radius: 14px; font-size: 28px;
  background: linear-gradient(135deg, rgba(24,160,88,0.12), rgba(112,192,232,0.12));
  border: 2px solid rgba(24,160,88,0.2);
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.avatar-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.avatar-option {
  width: 34px; height: 34px; border-radius: 8px; font-size: 18px;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; border: 2px solid transparent;
  transition: border-color 0.2s, background 0.2s;
  background: rgba(128,128,128,0.06);
}
.avatar-option:hover { background: rgba(128,128,128,0.12); }
.avatar-option.selected { border-color: var(--sre-primary); background: rgba(24,160,88,0.08); }
.modal-footer {
  display: flex; justify-content: flex-end;
  padding-top: 16px; margin-top: 4px;
  border-top: 1px solid var(--sre-border);
}

/* Notify config list */
.notify-config-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 4px;
}
.notify-config-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background: rgba(128, 128, 128, 0.06);
  border-radius: 8px;
}
.notify-config-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}
.notify-config-value {
  font-size: 12px;
  color: var(--sre-text-secondary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

