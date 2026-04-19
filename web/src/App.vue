<script setup lang="ts">
import {
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  NNotificationProvider,
  darkTheme,
} from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import { ref, provide, watch, onMounted, computed } from 'vue'
import AuroraBackground from '@/components/common/AuroraBackground.vue'
// SpotlightCursor removed in v1.8.1 — mousemove → reactive inline style was
// forcing a full-viewport radial-gradient repaint on every cursor frame,
// causing visible input lag and menu-click white-screens.

const savedTheme = localStorage.getItem('sre-theme')
const isDark = ref(savedTheme ? savedTheme === 'dark' : true)
const theme = computed(() => isDark.value ? darkTheme : null)

// Shared across both themes — gives Naive UI a consistent brand voice so
// buttons, tabs, switches, and focus rings all pick up the SREAgent accent.
const common = {
  primaryColor:        '#18a058',
  primaryColorHover:   '#22c55e',
  primaryColorPressed: '#138a4b',
  primaryColorSuppl:   '#22c55e',
  errorColor:          '#ef4444',
  errorColorHover:     '#f87171',
  errorColorPressed:   '#dc2626',
  warningColor:        '#f59e0b',
  warningColorHover:   '#fbbf24',
  warningColorPressed: '#d97706',
  infoColor:           '#3b82f6',
  infoColorHover:      '#60a5fa',
  infoColorPressed:    '#2563eb',
  successColor:        '#10b981',
  successColorHover:   '#34d399',
  successColorPressed: '#059669',
  borderRadius:        '10px',
  borderRadiusSmall:   '6px',
  fontFamily:
    '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", Roboto, "Helvetica Neue", Arial, sans-serif',
  fontFamilyMono:
    '"SF Mono", "JetBrains Mono", "Fira Code", ui-monospace, Consolas, Menlo, monospace',
}

const darkOverrides: GlobalThemeOverrides = {
  common: {
    ...common,
    bodyColor:     '#0b0e14',
    cardColor:     '#121722',
    modalColor:    '#192030',
    popoverColor:  '#192030',
    tableColor:    '#121722',
    tableColorHover: 'rgba(255,255,255,0.04)',
    borderColor:   'rgba(255,255,255,0.08)',
    dividerColor:  'rgba(255,255,255,0.08)',
    hoverColor:    'rgba(255,255,255,0.05)',
    textColorBase:      'rgba(255,255,255,0.92)',
    textColor1:         'rgba(255,255,255,0.92)',
    textColor2:         'rgba(255,255,255,0.78)',
    textColor3:         'rgba(255,255,255,0.58)',
    textColorDisabled:  'rgba(255,255,255,0.26)',
    placeholderColor:   'rgba(255,255,255,0.34)',
  },
  Card: {
    color:         '#121722',
    colorEmbedded: '#0f1420',
    borderColor:   'rgba(255,255,255,0.08)',
    borderRadius:  '14px',
  },
  Button: {
    borderRadiusMedium: '10px',
    borderRadiusSmall:  '8px',
    borderRadiusTiny:   '6px',
    fontWeight:         '500',
  },
  DataTable: {
    thColor:           '#0f1420',
    tdColor:           '#121722',
    tdColorHover:      'rgba(255,255,255,0.03)',
    borderColor:       'rgba(255,255,255,0.06)',
    borderRadius:      '12px',
  },
  Layout: {
    color:       '#0b0e14',
    siderColor:  '#0d1119',
    headerColor: '#121722',
  },
  Modal:  { color: '#192030' },
  Drawer: { color: '#192030' },
  Tag: {
    borderRadius: '6px',
  },
  Menu: {
    itemHeight:             '40px',
    borderRadius:           '8px',
    itemColorHover:         'rgba(255,255,255,0.05)',
    itemColorActive:        'rgba(24,160,88,0.14)',
    itemColorActiveHover:   'rgba(24,160,88,0.18)',
    itemTextColor:          'rgba(255,255,255,0.68)',
    itemTextColorHover:     'rgba(255,255,255,0.92)',
    itemTextColorActive:    '#22c55e',
    itemIconColorActive:    '#22c55e',
    itemIconColorActiveHover:'#22c55e',
  },
  Tabs: {
    tabBorderRadius: '8px',
    tabPaddingSmall: '6px 12px',
  },
  Input: {
    borderRadius: '8px',
  },
}

const lightOverrides: GlobalThemeOverrides = {
  common: {
    ...common,
    bodyColor:     '#f3f5f8',
    cardColor:     '#ffffff',
    modalColor:    '#ffffff',
    popoverColor:  '#ffffff',
    tableColor:    '#ffffff',
    tableColorHover: 'rgba(15,23,42,0.03)',
    borderColor:   '#e4e7ec',
    dividerColor:  '#e4e7ec',
    hoverColor:    'rgba(15,23,42,0.04)',
    textColorBase: 'rgba(15,23,42,0.92)',
    textColor1:    'rgba(15,23,42,0.92)',
    textColor2:    'rgba(15,23,42,0.72)',
    textColor3:    'rgba(15,23,42,0.52)',
  },
  Card: {
    color:         '#ffffff',
    colorEmbedded: '#f7f8fa',
    borderColor:   '#e4e7ec',
    borderRadius:  '14px',
  },
  Button: {
    borderRadiusMedium: '10px',
    borderRadiusSmall:  '8px',
    borderRadiusTiny:   '6px',
    fontWeight:         '500',
  },
  DataTable: {
    tdColor:      '#ffffff',
    thColor:      '#f7f8fa',
    tdColorHover: 'rgba(15,23,42,0.03)',
    borderColor:  '#edeff3',
    borderRadius: '12px',
  },
  Layout: {
    color:       '#f3f5f8',
    siderColor:  '#ffffff',
    headerColor: '#ffffff',
  },
  Modal:  { color: '#ffffff' },
  Drawer: { color: '#ffffff' },
  Tag:    { borderRadius: '6px' },
  Menu: {
    itemHeight:             '40px',
    borderRadius:           '8px',
    itemColorHover:         'rgba(15,23,42,0.04)',
    itemColorActive:        'rgba(24,160,88,0.10)',
    itemColorActiveHover:   'rgba(24,160,88,0.14)',
    itemTextColor:          'rgba(15,23,42,0.70)',
    itemTextColorHover:     'rgba(15,23,42,0.92)',
    itemTextColorActive:    '#138a4b',
    itemIconColorActive:    '#138a4b',
    itemIconColorActiveHover:'#138a4b',
  },
  Tabs: {
    tabBorderRadius: '8px',
    tabPaddingSmall: '6px 12px',
  },
  Input: {
    borderRadius: '8px',
  },
  Select: {
    peers: {
      InternalSelectMenu: { color: '#ffffff' },
    },
  },
}

const themeOverrides = computed<GlobalThemeOverrides>(() =>
  isDark.value ? darkOverrides : lightOverrides
)

function applyBodyClass(dark: boolean) {
  if (dark) {
    document.body.classList.remove('light-theme')
  } else {
    document.body.classList.add('light-theme')
  }
}

onMounted(() => {
  applyBodyClass(isDark.value)
})

watch(isDark, (val) => {
  localStorage.setItem('sre-theme', val ? 'dark' : 'light')
  applyBodyClass(val)
})

provide('toggleTheme', () => {
  isDark.value = !isDark.value
})

// View Transitions API CSS keyframes live in global.css.
// The existing Vue <transition name="sre-page"> provides the actual animation;
// browsers that support ::view-transition-* will additionally enhance it.
provide('isDark', isDark)
</script>

<template>
  <NConfigProvider :theme="theme" :theme-overrides="themeOverrides">
    <AuroraBackground />
    <NMessageProvider placement="top-right" :duration="2800" :max="4">
      <NDialogProvider>
        <NNotificationProvider placement="top-right" :max="4">
          <!-- No outer transition here — MainLayout wraps router-view in its
               own page transition. Running two nested <transition> with
               mode="out-in" was causing the "white screen on menu click"
               flash because the outer wrapper waited for the inner to
               complete its leave+enter cycle before re-mounting. -->
          <router-view />
        </NNotificationProvider>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style>
body {
  margin: 0;
  padding: 0;
}
</style>
