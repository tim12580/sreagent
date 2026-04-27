<script setup lang="ts">
import { ref, computed, watch, h, inject, onMounted, onUnmounted } from 'vue'
import CommandPalette from '@/components/common/CommandPalette.vue'
import { useCommandPalette } from '@/composables/useCommandPalette'
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
  ChevronBackOutline,
  ChevronForwardOutline,
} from '@vicons/ionicons5'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t, locale } = useI18n()
const message = useMessage()

// Sidebar collapse state: always-visible, user-controlled.
// Defaults to expanded; the «/» chevron button toggles into icon-rail
// (64px) mode. We removed the pin/hover-to-expand dance — users found
// it jumpy, and "expanded by default + one-click collapse" is the
// pattern nearly every SaaS app uses.
const collapsed = ref(JSON.parse(localStorage.getItem('sre-sider-collapsed') ?? 'false'))
watch(collapsed, v => localStorage.setItem('sre-sider-collapsed', JSON.stringify(v)))

function toggleCollapsed() {
  collapsed.value = !collapsed.value
}

const { open: openPalette } = useCommandPalette()

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
    {
      label: t('menu.datasources'), key: '/datasources', icon: renderIcon(ServerOutline),
      children: [
        { label: t('menu.datasourceList'), key: '/datasources' },
        { label: t('menu.datasourceQuery'), key: '/datasources/query' },
      ],
    },
    {
      label: t('menu.alertManagement'),  key: '/alerts', icon: renderIcon(AlertCircleOutline),
      children: [
        { label: t('menu.alertRules'),   key: '/alerts/rules' },
        { label: t('menu.activeAlerts'), key: '/alerts/events' },
        { label: t('menu.alertHistory'), key: '/alerts/history' },
        { label: t('menu.muteRules'),       key: '/alerts/mute-rules' },
        { label: t('menu.inhibitionRules'), key: '/alerts/inhibition-rules' },
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
  if (p.startsWith('/datasources/query'))         return '/datasources/query'
  if (p.startsWith('/datasources'))               return '/datasources'
  if (p.startsWith('/alerts/rules'))              return '/alerts/rules'
  if (p.startsWith('/alerts/events'))             return '/alerts/events'
  if (p.startsWith('/alerts/history'))            return '/alerts/history'
  if (p.startsWith('/alerts/mute-rules'))         return '/alerts/mute-rules'
  if (p.startsWith('/alerts/inhibition-rules'))   return '/alerts/inhibition-rules'
  if (p.startsWith('/notification'))              return '/notification'
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

// True when the saved avatar is an uploaded image (data: URL or http(s) URL),
// false for emoji presets / empty values.
function isImageAvatar(v: string | undefined | null): boolean {
  if (!v) return false
  return v.startsWith('data:image/') || v.startsWith('http://') || v.startsWith('https://') || v.startsWith('/')
}
const headerAvatar = computed(() => authStore.user?.avatar || '')
const headerAvatarIsImage = computed(() => isImageAvatar(headerAvatar.value))

// ===== Breadcrumb =====
const pageTitle = computed(() => {
  const p = route.path
  if (p === '/dashboard')                         return t('menu.dashboard')
  if (p === '/datasources')                       return t('menu.datasources')
  if (p.startsWith('/datasources/query'))         return t('menu.datasourceQuery')
  if (p.startsWith('/alerts/rules'))              return t('menu.alertRules')
  if (p.startsWith('/alerts/events'))             return t('menu.activeAlerts')
  if (p.startsWith('/alerts/history'))            return t('menu.alertHistory')
  if (p.startsWith('/alerts/mute-rules'))         return t('menu.muteRules')
  if (p.startsWith('/alerts/inhibition-rules'))   return t('menu.inhibitionRules')
  if (p.startsWith('/notification'))              return t('menu.notification')
  if (p === '/schedule')                          return t('menu.schedule')
  if (p === '/settings')                          return t('menu.settings')
  return ''
})
const parentTitle = computed(() => {
  const p = route.path
  if (p.startsWith('/datasources/'))   return t('menu.datasources')
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

// Preset avatars (emoji-based, stored as plain text).
// For custom images we store a base64 data: URL in the same `avatar` column.
const presetAvatars = [
  '👤','🧑‍💻','👩‍💻','🧑‍🔧','👩‍🔧','🧑‍🚀','👩‍🚀','🧑‍🔬','👩‍🔬',
  '🧑‍💼','👩‍💼','🧑‍🎤','🧑‍🎨','🦊','🐺','🐧','🦅','🦁','🐯','🐻',
  '🐼','🦉','🦄','🐉','🤖','👾','🛰️','🚀','⚡','🔥','🌟','🌈',
]

// Custom upload: base64-encoded data URL, capped at 200 KB.
const AVATAR_MAX_BYTES = 200 * 1024
const avatarFileInput = ref<HTMLInputElement | null>(null)

function triggerAvatarUpload() {
  avatarFileInput.value?.click()
}

function onAvatarFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!/^image\/(png|jpe?g|svg\+xml|webp)$/.test(file.type)) {
    message.error(t('profile.avatarInvalidType'))
    input.value = ''
    return
  }
  if (file.size > AVATAR_MAX_BYTES) {
    message.error(t('profile.avatarTooLarge'))
    input.value = ''
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    profileForm.value.avatar = String(reader.result || '')
  }
  reader.onerror = () => message.error(t('common.failed'))
  reader.readAsDataURL(file)
  input.value = ''
}

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

// ===== Lark Bind =====
const larkOpenIdInput = ref('')
const larkBindSaving = ref(false)

async function saveLarkBind() {
  const openId = larkOpenIdInput.value.trim()
  if (!openId) return
  larkBindSaving.value = true
  try {
    await authApi.bindLark(openId)
    message.success(t('settings.larkBindSuccess'))
    larkOpenIdInput.value = ''
  } catch (err: any) {
    message.error(err.message || t('common.failed'))
  } finally {
    larkBindSaving.value = false
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

    <!-- ===== Sidebar (always visible, user-togglable collapse) ===== -->
    <n-layout-sider
      class="sre-sider"
      bordered
      collapse-mode="width"
      :collapsed-width="64"
      :width="224"
      :collapsed="collapsed"
      :native-scrollbar="false"
    >
      <!-- Logo area -->
      <div class="sider-logo" :class="{ collapsed }">
        <img src="/logo.svg" alt="SREAgent" class="logo-mark" />
        <transition name="fade">
          <span v-if="!collapsed" class="logo-text">
            <span class="gradient-text">SRE</span>Agent
          </span>
        </transition>
      </div>

      <!-- Navigation menu -->
      <n-menu
        class="sre-menu"
        :collapsed="collapsed"
        :collapsed-width="64"
        :collapsed-icon-size="22"
        :indent="18"
        :options="menuOptions"
        :value="menuSelectedKey"
        @update:value="handleMenuClick"
      />

      <!-- Bottom: collapse toggle + version -->
      <div class="sider-bottom">
        <div
          class="sider-collapse-toggle"
          :class="{ collapsed }"
          :title="collapsed ? t('header.expandSidebar') : t('header.collapseSidebar')"
          @click="toggleCollapsed"
        >
          <n-icon
            :component="collapsed ? ChevronForwardOutline : ChevronBackOutline"
            :size="16"
            class="collapse-icon"
            :class="{ rotated: collapsed }"
          />
          <transition name="fade">
            <span v-if="!collapsed" class="collapse-label">{{ t('header.collapseSidebar') }}</span>
          </transition>
        </div>
        <transition name="fade">
          <div v-if="!collapsed" class="sider-version">v{{ __APP_VERSION__ }}</div>
        </transition>
      </div>
    </n-layout-sider>

    <CommandPalette />

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

          <!-- ① Search / ⌘K -->
          <div class="ctrl-btn ctrl-btn--search" @click="openPalette" title="⌘K">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
            </svg>
            <kbd class="cmd-shortcut">⌘K</kbd>
          </div>

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
              <div class="user-avatar" :class="{ 'user-avatar--image': headerAvatarIsImage, 'user-avatar--emoji': !!headerAvatar && !headerAvatarIsImage }">
                <img v-if="headerAvatarIsImage" :src="headerAvatar" alt="avatar" />
                <template v-else-if="headerAvatar">{{ headerAvatar }}</template>
                <template v-else>{{ userInitial }}</template>
              </div>
              <span class="user-name">{{ displayName }}</span>
              <n-icon :component="ChevronDownOutline" :size="12" class="user-chevron" />
            </div>
          </n-dropdown>

        </div>
      </div>

      <!-- Main content -->
      <!-- v1.8.1: dropped <transition name="page" mode="out-in"> — the
           opacity-0 + translateY leave state of the previous page rendered
           as a blank viewport for ~180ms on every menu click, which users
           perceived as "menu click does nothing / white screen". The
           Naive UI + Vue Router handoff is already smooth enough without
           an extra page-level transition, and child components can opt
           into their own entrance animation if needed. -->
      <n-layout-content class="sre-content" :native-scrollbar="false">
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
            <img v-if="isImageAvatar(profileForm.avatar)" :src="profileForm.avatar" alt="avatar" class="avatar-preview-img" />
            <span v-else class="avatar-preview">{{ profileForm.avatar || userInitial }}</span>
          </div>
          <div class="avatar-actions">
            <div class="avatar-grid">
              <span
                v-for="a in presetAvatars"
                :key="a"
                class="avatar-option"
                :class="{ selected: profileForm.avatar === a }"
                @click="profileForm.avatar = a"
              >{{ a }}</span>
            </div>
            <div class="avatar-upload-row">
              <n-button size="tiny" secondary @click="triggerAvatarUpload">
                📎 {{ t('profile.uploadAvatar') }}
              </n-button>
              <n-button
                v-if="profileForm.avatar"
                size="tiny"
                quaternary
                type="error"
                @click="profileForm.avatar = ''"
              >
                {{ t('profile.clearAvatar') }}
              </n-button>
              <input
                ref="avatarFileInput"
                type="file"
                accept="image/png,image/jpeg,image/svg+xml,image/webp"
                style="display: none"
                @change="onAvatarFileChange"
              />
            </div>
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

      <!-- Tab 4: Lark Bind -->
      <n-tab-pane name="lark" :tab="t('settings.larkBind')">
        <n-space vertical size="large" style="padding: 8px 0">
          <n-alert type="info" :title="t('settings.larkBind')" style="font-size:13px">
            {{ t('settings.larkBindHint') }}
          </n-alert>
          <n-form label-placement="top" size="small">
            <n-form-item :label="t('settings.larkOpenId')">
              <n-input
                v-model:value="larkOpenIdInput"
                :placeholder="t('settings.larkOpenId')"
                clearable
                style="max-width: 360px"
              />
            </n-form-item>
          </n-form>
          <n-button type="primary" :loading="larkBindSaving" :disabled="!larkOpenIdInput.trim()" @click="saveLarkBind">
            {{ t('settings.larkBind') }}
          </n-button>
        </n-space>
      </n-tab-pane>

    </n-tabs>
  </n-modal>
</template>

<style scoped>
/* ===== Icon Rail Sidebar ===== */
.sre-sider {
  background:
    linear-gradient(180deg, rgba(24,160,88,0.06) 0%, transparent 50%),
    var(--sre-bg-card);
  border-right: 1px solid var(--sre-border);
  position: relative;
  /* Smooth width transition for hover expand */
  transition: width 220ms var(--sre-ease-out) !important;
}

/* Noise grain on sider */
.sre-sider::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: var(--sre-noise-url);
  opacity: 0.03;
  mix-blend-mode: overlay;
  pointer-events: none;
  z-index: 0;
}

.sider-logo {
  display: flex;
  align-items: center;
  gap: var(--sre-space-3);
  padding: 14px 18px;
  height: 60px;
  border-bottom: 1px solid var(--sre-border);
  transition: padding var(--sre-duration-base) var(--sre-ease-out);
  position: relative;
  z-index: 1;
}
.sider-logo.collapsed {
  justify-content: center;
  padding: 14px 12px;
}

/* Collapse / expand chevron — floats on the outer edge of the sider,
   straddling the border so it reads as an attached "tab". Always visible,
   no hover-reveal, consistent with VSCode / Linear / Notion patterns. */
.logo-mark {
  width: 32px;
  height: 32px;
  border-radius: var(--sre-radius-md);
  flex-shrink: 0;
  display: block;
  filter: drop-shadow(0 4px 16px rgba(24, 160, 88, 0.50));
  position: relative;
}
.logo-text {
  font-size: var(--sre-fs-lg);
  font-weight: var(--sre-fw-semibold);
  letter-spacing: -0.01em;
  white-space: nowrap;
  color: var(--sre-text-primary);
}

.sre-menu {
  padding: var(--sre-space-2) var(--sre-space-2);
  position: relative;
  z-index: 1;
}

/* Active menu item — left accent bar */
.sre-menu :deep(.n-menu-item-content--selected)::before {
  content: '';
  position: absolute;
  left: 0;
  top: 6px;
  bottom: 6px;
  width: 3px;
  border-radius: 0 3px 3px 0;
  background: var(--sre-gradient-brand);
}
.sre-menu :deep(.n-menu-item-content) {
  position: relative;
  overflow: visible;
}

/* Bottom area: ⌘K + version */
.sider-bottom {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  padding: var(--sre-space-3);
  border-top: 1px solid var(--sre-border);
  display: flex;
  flex-direction: column;
  gap: var(--sre-space-2);
  background: var(--sre-bg-card);
  z-index: 1;
}

.sider-collapse-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: var(--sre-radius-md);
  cursor: pointer;
  transition: background var(--sre-duration-base) var(--sre-ease-out),
              color var(--sre-duration-base) var(--sre-ease-out);
  color: var(--sre-text-tertiary);
  user-select: none;
  white-space: nowrap;
  overflow: hidden;
}
.sider-collapse-toggle.collapsed {
  justify-content: center;
}
.sider-collapse-toggle:hover {
  background: var(--sre-primary-soft);
  color: var(--sre-primary);
}
.collapse-icon {
  flex-shrink: 0;
  transition: transform var(--sre-duration-base) var(--sre-ease-spring);
}
.collapse-icon.rotated {
  transform: rotate(180deg);
}
.collapse-label {
  font-size: var(--sre-fs-sm);
  font-weight: var(--sre-fw-medium);
}

.sider-version {
  text-align: center;
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-secondary);
  letter-spacing: 0.05em;
  padding: 4px 0 2px;
  opacity: 0.7;
}

/* ⌘K trigger in header */
.ctrl-btn--search {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border-radius: var(--sre-radius-pill);
  border: 1px solid var(--sre-border);
  background: var(--sre-bg-sunken);
  color: var(--sre-text-tertiary);
  cursor: pointer;
  transition: background var(--sre-duration-base) var(--sre-ease-out),
              border-color var(--sre-duration-base) var(--sre-ease-out),
              color var(--sre-duration-base) var(--sre-ease-out);
  font-size: var(--sre-fs-sm);
}
.ctrl-btn--search:hover {
  background: var(--sre-primary-soft);
  border-color: var(--sre-primary-ring);
  color: var(--sre-primary);
}
.cmd-shortcut {
  font-size: var(--sre-fs-2xs);
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--sre-bg-elevated);
  border: 1px solid var(--sre-border-strong);
  color: var(--sre-text-muted);
  font-family: var(--sre-font-mono);
  pointer-events: none;
}

/* ===== Header bar ===== */
.header-bar {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--sre-space-6);
  border-bottom: 1px solid var(--sre-border);
  /* v1.8.1: dropped backdrop-filter — it was the single most expensive
     GPU op in the app and the header is always in the composited tree,
     so it forced a blur of the entire page on every scroll/repaint.
     Solid tinted background reads identically and composites for free. */
  background: var(--sre-bg-card);
  flex-shrink: 0;
  position: sticky;
  top: 0;
  z-index: var(--sre-z-sticky);
  transition: background var(--sre-duration-slow) var(--sre-ease-out),
              border-color var(--sre-duration-slow) var(--sre-ease-out);
}
.header-left {
  display: flex;
  align-items: center;
}
.header-page-title {
  font-size: var(--sre-fs-md);
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-primary);
  letter-spacing: -0.005em;
}
.header-right {
  display: flex;
  align-items: center;
  gap: var(--sre-space-1);
}

/* ===== Content shell ===== */
.sre-content {
  padding: var(--sre-space-6);
  background: transparent;
}
.sre-content :deep(.n-scrollbar-content) {
  min-height: calc(100vh - 60px);
}

/* Route transition removed — see template comment. */

/* Subtle separator */
.header-sep {
  width: 1px;
  height: 18px;
  background: var(--sre-border);
  margin: 0 var(--sre-space-2);
  opacity: 0.7;
}

/* ===== Clock Pill ===== */
.clock-pill {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
  padding: 6px 12px;
  border-radius: var(--sre-radius-pill);
  cursor: pointer;
  user-select: none;
  transition: background var(--sre-duration-base) var(--sre-ease-out),
              border-color var(--sre-duration-base) var(--sre-ease-out),
              box-shadow var(--sre-duration-base) var(--sre-ease-out);
  border: 1px solid var(--sre-border);
  background: var(--sre-bg-sunken);
}
.clock-pill:hover,
.clock-pill.active {
  background: var(--sre-primary-soft);
  border-color: var(--sre-primary-ring);
  box-shadow: 0 0 0 3px var(--sre-primary-soft);
}
.clock-icon {
  color: var(--sre-primary);
  flex-shrink: 0;
}
.clock-time {
  font-family: var(--sre-font-mono);
  font-size: var(--sre-fs-md);
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-primary);
  letter-spacing: 0.6px;
  font-feature-settings: "tnum" 1;
  transition: opacity var(--sre-duration-fast);
}
.clock-sep {
  color: var(--sre-text-tertiary);
  font-size: var(--sre-fs-xs);
}
.clock-date {
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-secondary);
  letter-spacing: 0.2px;
}
.clock-tz {
  font-size: var(--sre-fs-2xs);
  font-weight: var(--sre-fw-bold);
  color: var(--sre-primary);
  background: var(--sre-primary-soft);
  padding: 2px 7px;
  border-radius: var(--sre-radius-xs);
  letter-spacing: 0.6px;
  text-transform: uppercase;
}

/* ===== Timezone panel ===== */
.tz-panel {
  min-width: 240px;
  padding: var(--sre-space-2) 0;
  border-radius: var(--sre-radius-md);
}
.tz-panel-title {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
  padding: var(--sre-space-2) var(--sre-space-4) var(--sre-space-2);
  font-size: var(--sre-fs-xs);
  font-weight: var(--sre-fw-semibold);
  color: var(--sre-text-tertiary);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  border-bottom: 1px solid var(--sre-border);
  margin-bottom: var(--sre-space-1);
}
.tz-option {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
  padding: 8px var(--sre-space-4);
  cursor: pointer;
  font-size: var(--sre-fs-md);
  transition: background var(--sre-duration-fast) var(--sre-ease-out);
  color: var(--sre-text-primary);
  border-radius: var(--sre-radius-sm);
  margin: 0 var(--sre-space-2);
}
.tz-option:hover { background: var(--sre-bg-hover); }
.tz-option.selected {
  color: var(--sre-primary);
  background: var(--sre-primary-soft);
}
.tz-opt-abbr {
  font-weight: var(--sre-fw-bold);
  font-size: var(--sre-fs-xs);
  width: 36px;
  color: var(--sre-primary);
  flex-shrink: 0;
  letter-spacing: 0.04em;
}
.tz-opt-label { flex: 1; }
.tz-opt-check {
  font-size: var(--sre-fs-sm);
  color: var(--sre-primary);
  font-weight: var(--sre-fw-bold);
}

/* ===== Control buttons (lang / theme) ===== */
.ctrl-btn {
  display: inline-flex;
  align-items: center;
  gap: var(--sre-space-1);
  padding: 7px 10px;
  min-height: 34px;
  border-radius: var(--sre-radius-md);
  cursor: pointer;
  color: var(--sre-text-secondary);
  transition: background var(--sre-duration-base) var(--sre-ease-out),
              color var(--sre-duration-base) var(--sre-ease-out),
              transform var(--sre-duration-base) var(--sre-ease-out);
  font-size: var(--sre-fs-md);
  font-weight: var(--sre-fw-medium);
}
.ctrl-btn:hover {
  background: var(--sre-bg-hover);
  color: var(--sre-text-primary);
}
.ctrl-btn:active { transform: scale(0.96); }
.ctrl-label {
  font-size: var(--sre-fs-sm);
  font-weight: var(--sre-fw-semibold);
  letter-spacing: 0.4px;
}

/* ===== User pill ===== */
.user-pill {
  display: flex;
  align-items: center;
  gap: var(--sre-space-2);
  padding: 4px 12px 4px 4px;
  border-radius: var(--sre-radius-pill);
  cursor: pointer;
  transition: background var(--sre-duration-base) var(--sre-ease-out),
              border-color var(--sre-duration-base) var(--sre-ease-out),
              box-shadow var(--sre-duration-base) var(--sre-ease-out);
  border: 1px solid var(--sre-border);
  background: var(--sre-bg-sunken);
}
.user-pill:hover {
  background: var(--sre-primary-soft);
  border-color: var(--sre-primary-ring);
}
.user-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--sre-gradient-brand);
  color: #fff;
  font-size: var(--sre-fs-sm);
  font-weight: var(--sre-fw-bold);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
  box-shadow: 0 2px 8px -2px rgba(24, 160, 88, 0.45),
              inset 0 1px 0 rgba(255,255,255,0.2);
}
.user-avatar--emoji {
  font-size: 16px;
  font-weight: 400;
  line-height: 1;
  background: transparent;
  box-shadow: inset 0 0 0 1px var(--sre-border);
}
.user-avatar--image {
  background: transparent;
  box-shadow: inset 0 0 0 1px var(--sre-border);
}
.user-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.user-name {
  font-size: var(--sre-fs-md);
  font-weight: var(--sre-fw-medium);
  color: var(--sre-text-primary);
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.user-chevron {
  color: var(--sre-text-tertiary);
  flex-shrink: 0;
  transition: transform var(--sre-duration-base) var(--sre-ease-out);
}
.user-pill:hover .user-chevron { transform: translateY(1px); }

/* ===== Transitions ===== */
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* v1.8.1: removed first-mount animations on sider/header/content.
   The `animation-delay: 80ms` on .sre-content was showing an empty-looking
   page for the delay duration whenever the layout re-mounted (e.g. after
   hot-reload or full refresh), reinforcing the "app feels slow" complaint. */

/* Logo text smooth appear on uncollapse */
.logo-text {
  transition: opacity var(--sre-duration-base) var(--sre-ease-out),
              transform var(--sre-duration-base) var(--sre-ease-out);
}

/* Breadcrumb page title shimmer-in on route change */
.header-page-title {
  transition: opacity var(--sre-duration-fast) var(--sre-ease-out),
              transform var(--sre-duration-fast) var(--sre-ease-out);
}

/* ===== Profile Modal ===== */
.avatar-section { display: flex; align-items: flex-start; gap: 16px; padding: 12px 0 4px; }
.avatar-current {
  width: 60px; height: 60px; border-radius: 14px; font-size: 30px;
  background: linear-gradient(135deg, rgba(24,160,88,0.12), rgba(112,192,232,0.12));
  border: 2px solid rgba(24,160,88,0.2);
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
  overflow: hidden;
}
.avatar-preview-img {
  width: 100%; height: 100%; object-fit: cover; display: block;
}
.avatar-actions {
  flex: 1; min-width: 0;
  display: flex; flex-direction: column; gap: 10px;
}
.avatar-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(32px, 1fr));
  gap: 6px;
  max-height: 120px;
  overflow-y: auto;
  padding: 2px;
}
.avatar-option {
  width: 32px; height: 32px; border-radius: 8px; font-size: 17px;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; border: 2px solid transparent;
  transition: border-color 0.2s, background 0.2s, transform 0.15s;
  background: rgba(128,128,128,0.06);
}
.avatar-option:hover { background: rgba(128,128,128,0.12); transform: translateY(-1px); }
.avatar-option.selected { border-color: var(--sre-primary); background: rgba(24,160,88,0.10); }
.avatar-upload-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
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

