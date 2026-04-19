import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'

export interface PaletteItem {
  id: string
  label: string
  hint?: string
  group: 'navigate' | 'action' | 'recent'
  icon?: string     // ionicons5 name string — rendered by caller
  action: () => void
}

const visible = ref(false)
const query = ref('')

// Registry: actions registered by external composables/components
const registeredActions = ref<PaletteItem[]>([])

export function useCommandPalette() {
  const router = useRouter()

  function open() {
    query.value = ''
    visible.value = true
  }
  function close() { visible.value = false }
  function toggle() { visible.value ? close() : open() }

  // ── Navigate items (all app routes) ──────────────────────────────────
  const navigateItems = computed<PaletteItem[]>(() => [
    { id: 'nav-dashboard',    label: 'Dashboard',          hint: 'Page',    group: 'navigate', icon: 'grid-outline',            action: () => router.push('/dashboard') },
    { id: 'nav-datasources',  label: 'Data Sources',       hint: 'Page',    group: 'navigate', icon: 'server-outline',          action: () => router.push('/datasources') },
    { id: 'nav-rules',        label: 'Alert Rules',        hint: 'Alerts',  group: 'navigate', icon: 'alert-circle-outline',    action: () => router.push('/alerts/rules') },
    { id: 'nav-events',       label: 'Active Alerts',      hint: 'Alerts',  group: 'navigate', icon: 'flash-outline',           action: () => router.push('/alerts/events') },
    { id: 'nav-history',      label: 'Alert History',      hint: 'Alerts',  group: 'navigate', icon: 'time-outline',            action: () => router.push('/alerts/history') },
    { id: 'nav-mute',         label: 'Mute Rules',         hint: 'Alerts',  group: 'navigate', icon: 'volume-mute-outline',     action: () => router.push('/alerts/mute-rules') },
    { id: 'nav-inhibition',   label: 'Inhibition Rules',   hint: 'Alerts',  group: 'navigate', icon: 'shield-outline',          action: () => router.push('/alerts/inhibition-rules') },
    { id: 'nav-notification', label: 'Notifications',      hint: 'Page',    group: 'navigate', icon: 'notifications-outline',   action: () => router.push('/notification') },
    { id: 'nav-schedule',     label: 'On-Call Schedule',   hint: 'Page',    group: 'navigate', icon: 'calendar-outline',        action: () => router.push('/schedule') },
    { id: 'nav-settings',     label: 'Settings',           hint: 'Page',    group: 'navigate', icon: 'settings-outline',        action: () => router.push('/settings') },
  ])

  // ── Recent (last 5 navigations from localStorage) ────────────────────
  const RECENT_KEY = 'sre-cmd-recent'
  function getRecent(): PaletteItem[] {
    try {
      const ids: string[] = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]')
      return ids
        .map(id => navigateItems.value.find(i => i.id === id))
        .filter(Boolean)
        .map(i => ({ ...i!, group: 'recent' as const }))
    } catch { return [] }
  }

  function pushRecent(id: string) {
    try {
      const ids: string[] = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]')
      const next = [id, ...ids.filter(x => x !== id)].slice(0, 5)
      localStorage.setItem(RECENT_KEY, JSON.stringify(next))
    } catch { /**/ }
  }

  function runItem(item: PaletteItem) {
    if (item.group === 'navigate' || item.group === 'recent') {
      pushRecent(item.id)
    }
    close()
    item.action()
  }

  // ── Fuzzy filter ─────────────────────────────────────────────────────
  function score(text: string, q: string): number {
    const t = text.toLowerCase()
    const ql = q.toLowerCase()
    if (!ql) return 1
    if (t === ql) return 100
    if (t.startsWith(ql)) return 80
    if (t.includes(ql)) return 60
    // word-boundary: any word starts with q
    const words = t.split(/[\s\-_/]+/)
    if (words.some(w => w.startsWith(ql))) return 50
    // character subsequence
    let ci = 0
    for (const ch of ql) {
      const idx = t.indexOf(ch, ci)
      if (idx === -1) return 0
      ci = idx + 1
    }
    return 20
  }

  const filteredItems = computed(() => {
    const q = query.value.trim()
    const recent = getRecent()

    if (!q) {
      return {
        recent: recent.slice(0, 5),
        navigate: navigateItems.value.slice(0, 8),
        action: registeredActions.value.slice(0, 6),
      }
    }

    const filter = (items: PaletteItem[]) =>
      items
        .map(i => ({ item: i, s: Math.max(score(i.label, q), score(i.hint || '', q)) }))
        .filter(x => x.s > 0)
        .sort((a, b) => b.s - a.s)
        .map(x => x.item)

    return {
      recent: [],
      navigate: filter(navigateItems.value),
      action: filter(registeredActions.value),
    }
  })

  function registerAction(item: Omit<PaletteItem, 'group'>) {
    // De-dup: CommandPalette.vue's onMounted registers built-in actions on
    // every remount (HMR, route remount). Without this guard the actions
    // list doubled/tripled every hot reload.
    if (registeredActions.value.some(a => a.id === item.id)) return
    registeredActions.value.push({ ...item, group: 'action' })
  }

  return {
    visible,
    query,
    open,
    close,
    toggle,
    filteredItems,
    runItem,
    registerAction,
  }
}
