<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted, inject } from 'vue'
// computed imported above; ensures groupLabel reacts to locale switches
import type { Ref } from 'vue'
import { useCommandPalette, type PaletteItem } from '@/composables/useCommandPalette'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const {
  visible, query, close, filteredItems, runItem, registerAction,
} = useCommandPalette()

const isDark = inject<Ref<boolean>>('isDark', ref(true))
const toggleTheme = inject<() => void>('toggleTheme', () => {})

const inputRef = ref<HTMLInputElement | null>(null)
const activeIndex = ref(0)

// Register built-in actions once
onMounted(() => {
  registerAction({ id: 'act-theme', label: 'Toggle Dark / Light Mode', hint: 'Action', icon: 'contrast-outline', action: toggleTheme })
  registerAction({ id: 'act-lang-en', label: 'Switch to English', hint: 'Action', icon: 'language-outline', action: () => { /* handled elsewhere */ } })
})

// Flat list of all visible items for keyboard nav
const allItems = computed<PaletteItem[]>(() => {
  const f = filteredItems.value
  return [...f.recent, ...f.navigate, ...f.action]
})

watch(visible, async (v) => {
  if (v) {
    activeIndex.value = 0
    await nextTick()
    inputRef.value?.focus()
  }
})

watch(query, () => { activeIndex.value = 0 })

function onKeydown(e: KeyboardEvent) {
  if (!visible.value) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    activeIndex.value = (activeIndex.value + 1) % Math.max(1, allItems.value.length)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    activeIndex.value = (activeIndex.value - 1 + Math.max(1, allItems.value.length)) % Math.max(1, allItems.value.length)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const item = allItems.value[activeIndex.value]
    if (item) runItem(item)
  } else if (e.key === 'Escape') {
    close()
  }
}

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault()
    visible.value ? close() : (visible.value = true)
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onUnmounted(() => window.removeEventListener('keydown', onGlobalKeydown))

// Group helpers — i18n-backed so zh/en both render correctly.
const groupLabel = computed<Record<string, string>>(() => ({
  recent:   t('palette.recent'),
  navigate: t('palette.navigate'),
  action:   t('palette.actions'),
}))

function sections() {
  const f = filteredItems.value
  const out: { key: string; label: string; items: PaletteItem[] }[] = []
  if (f.recent.length)   out.push({ key: 'recent',   label: groupLabel.value.recent,   items: f.recent })
  if (f.navigate.length) out.push({ key: 'navigate', label: groupLabel.value.navigate, items: f.navigate })
  if (f.action.length)   out.push({ key: 'action',   label: groupLabel.value.action,   items: f.action })
  return out
}

function globalIndex(sectionIdx: number, itemIdx: number) {
  const f = filteredItems.value
  const lens = [f.recent.length, f.navigate.length, f.action.length]
  let offset = 0
  for (let i = 0; i < sectionIdx; i++) offset += lens[i]
  return offset + itemIdx
}

function hintColor(item: PaletteItem) {
  if (item.group === 'recent') return 'var(--sre-text-tertiary)'
  if (item.hint === 'Action') return 'var(--sre-aurora-3)'
  return 'var(--sre-text-tertiary)'
}
</script>

<template>
  <teleport to="body">
    <transition name="cp-backdrop">
      <div v-if="visible" class="cp-backdrop" @click.self="close" />
    </transition>
    <transition name="cp-panel">
      <div v-if="visible" class="cp-panel conic-border noise-overlay" @keydown="onKeydown">
        <!-- Search input -->
        <div class="cp-search">
          <svg class="cp-search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
          </svg>
          <input
            ref="inputRef"
            v-model="query"
            class="cp-input"
            :placeholder="t('palette.searchPlaceholder')"
            autocomplete="off"
            spellcheck="false"
          />
          <kbd class="cp-esc" @click="close">Esc</kbd>
        </div>

        <!-- Results -->
        <div class="cp-body">
          <template v-if="allItems.length === 0">
            <div class="cp-empty">{{ t('palette.noResults', { q: query }) }}</div>
          </template>
          <template v-for="(section, si) in sections()" :key="section.key">
            <div class="cp-group-label">{{ section.label }}</div>
            <button
              v-for="(item, ii) in section.items"
              :key="item.id"
              class="cp-item"
              :class="{ active: activeIndex === globalIndex(si, ii) }"
              @mouseenter="activeIndex = globalIndex(si, ii)"
              @click="runItem(item)"
            >
              <span class="cp-item-label">{{ item.label }}</span>
              <span v-if="item.hint" class="cp-item-hint" :style="{ color: hintColor(item) }">{{ item.hint }}</span>
            </button>
          </template>
        </div>

        <!-- Footer -->
        <div class="cp-footer">
          <span class="cp-key-hint"><kbd>↑↓</kbd> {{ t('palette.navigate') }}</span>
          <span class="cp-key-hint"><kbd>↵</kbd> {{ t('palette.open') }}</span>
          <span class="cp-key-hint"><kbd>Esc</kbd> {{ t('palette.close') }}</span>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<style scoped>
.cp-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(4px);
  z-index: calc(var(--sre-z-modal) - 1);
}

.cp-panel {
  position: fixed;
  top: 18vh;
  left: 50%;
  transform: translateX(-50%);
  width: min(640px, 90vw);
  border-radius: var(--sre-radius-xl);
  background: color-mix(in srgb, var(--sre-bg-card) 60%, transparent);
  backdrop-filter: saturate(180%) blur(28px);
  -webkit-backdrop-filter: saturate(180%) blur(28px);
  box-shadow: var(--sre-shadow-soft-xl);
  z-index: var(--sre-z-modal);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  max-height: 60vh;
}
body.light-theme .cp-panel {
  background: color-mix(in srgb, #ffffff 80%, transparent);
}

/* ─── Search ──────────────────────────────────────────────── */
.cp-search {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--sre-border);
  flex-shrink: 0;
}
.cp-search-icon {
  color: var(--sre-text-tertiary);
  flex-shrink: 0;
}
.cp-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  font-size: var(--sre-fs-lg);
  color: var(--sre-text-primary);
  font-family: var(--sre-font-sans);
  caret-color: var(--sre-primary);
}
.cp-input::placeholder { color: var(--sre-text-muted); }
.cp-esc {
  flex-shrink: 0;
  font-size: var(--sre-fs-2xs);
  padding: 2px 7px;
  border-radius: var(--sre-radius-sm);
  background: var(--sre-bg-elevated);
  border: 1px solid var(--sre-border-strong);
  color: var(--sre-text-tertiary);
  cursor: pointer;
  font-family: var(--sre-font-sans);
  user-select: none;
  transition: background var(--sre-duration-fast) var(--sre-ease-out);
}
.cp-esc:hover { background: var(--sre-bg-hover); }

/* ─── Body ────────────────────────────────────────────────── */
.cp-body {
  overflow-y: auto;
  flex: 1;
  padding: 8px 0;
  scrollbar-width: thin;
  scrollbar-color: rgba(128,128,128,0.2) transparent;
}
.cp-empty {
  padding: 32px 20px;
  text-align: center;
  color: var(--sre-text-tertiary);
  font-size: var(--sre-fs-md);
}
.cp-group-label {
  padding: 6px 20px 4px;
  font-size: var(--sre-fs-2xs);
  font-weight: var(--sre-fw-semibold);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--sre-text-muted);
}
.cp-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 10px 20px;
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  transition: background var(--sre-duration-fast) var(--sre-ease-out);
  gap: 10px;
}
.cp-item.active,
.cp-item:hover {
  background: var(--sre-primary-soft);
}
.cp-item.active .cp-item-label {
  color: var(--sre-text-primary);
}
.cp-item-label {
  font-size: var(--sre-fs-md);
  color: var(--sre-text-secondary);
  font-weight: var(--sre-fw-medium);
  transition: color var(--sre-duration-fast);
}
.cp-item-hint {
  font-size: var(--sre-fs-xs);
  font-weight: var(--sre-fw-medium);
  white-space: nowrap;
  opacity: 0.75;
}

/* ─── Footer ──────────────────────────────────────────────── */
.cp-footer {
  display: flex;
  gap: 16px;
  padding: 10px 20px;
  border-top: 1px solid var(--sre-border);
  flex-shrink: 0;
}
.cp-key-hint {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: var(--sre-fs-xs);
  color: var(--sre-text-muted);
}
.cp-key-hint kbd {
  font-size: var(--sre-fs-2xs);
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--sre-bg-elevated);
  border: 1px solid var(--sre-border-strong);
  color: var(--sre-text-tertiary);
  font-family: var(--sre-font-mono);
}

/* ─── Transitions ─────────────────────────────────────────── */
.cp-backdrop-enter-active,
.cp-backdrop-leave-active { transition: opacity 180ms ease; }
.cp-backdrop-enter-from,
.cp-backdrop-leave-to { opacity: 0; }

.cp-panel-enter-active {
  transition: opacity 200ms var(--sre-ease-out),
              transform 200ms var(--sre-ease-spring);
}
.cp-panel-leave-active {
  transition: opacity 140ms ease-in,
              transform 140ms ease-in;
}
.cp-panel-enter-from,
.cp-panel-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-12px) scale(0.96);
}
</style>
