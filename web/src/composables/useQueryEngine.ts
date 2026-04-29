import { ref, type Ref } from 'vue'
import { datasourceApi } from '@/api'
import type { TimeRange, QueryTarget, QuerySeriesItem } from '@/types/query'

function autoStep(timeRange: TimeRange): string {
  const durationSec = (timeRange.end - timeRange.start) / 1000
  if (durationSec <= 300) return '15s'       // 5min
  if (durationSec <= 3600) return '30s'      // 1h
  if (durationSec <= 21600) return '1m'      // 6h
  if (durationSec <= 86400) return '5m'      // 24h
  if (durationSec <= 604800) return '15m'    // 7d
  return '1h'
}

function generateId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID()
  }
  // Fallback for non-secure contexts (HTTP)
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16)
  })
}

export function createDefaultTarget(): QueryTarget {
  return {
    id: generateId(),
    datasourceId: null,
    expression: '',
    legendFormat: '',
    enabled: true,
    state: 'idle',
    resultType: null,
    series: [],
    error: null,
  }
}

export function useQueryEngine(timeRange: Ref<TimeRange>) {
  const targets = ref<QueryTarget[]>([createDefaultTarget()])
  const globalLoading = ref(false)

  function addTarget() {
    const last = targets.value[targets.value.length - 1]
    targets.value.push({
      ...createDefaultTarget(),
      datasourceId: last?.datasourceId ?? null,
    })
  }

  function removeTarget(id: string) {
    if (targets.value.length <= 1) return
    targets.value = targets.value.filter(t => t.id !== id)
  }

  function toggleTarget(id: string) {
    const target = targets.value.find(t => t.id === id)
    if (target) target.enabled = !target.enabled
  }

  function updateTarget(id: string, patch: Partial<QueryTarget>) {
    const target = targets.value.find(t => t.id === id)
    if (target) Object.assign(target, patch)
  }

  async function executeQuery(target: QueryTarget) {
    if (!target.datasourceId || !target.expression.trim()) return

    target.state = 'loading'
    target.error = null
    target.series = []
    target.resultType = null

    try {
      const tr = timeRange.value
      const durationMs = tr.end - tr.start
      const isRange = durationMs > 60000 // > 1min → range query

      if (isRange) {
        const step = autoStep(tr)
        const res = await datasourceApi.rangeQuery(target.datasourceId, {
          expression: target.expression,
          start: Math.floor(tr.start / 1000),
          end: Math.floor(tr.end / 1000),
          step,
        })
        const data = res.data.data
        target.resultType = data.result_type as 'vector' | 'matrix'
        target.series = data.series || []
      } else {
        const res = await datasourceApi.query(target.datasourceId, {
          expression: target.expression,
          time: tr.end / 1000,
        })
        const data = res.data.data
        target.resultType = data.result_type as 'vector' | 'matrix'
        target.series = data.series || []
      }

      target.state = 'idle'
    } catch (err: any) {
      target.state = 'error'
      target.error = err?.response?.data?.message || err?.message || 'Query failed'
    }
  }

  async function executeAll() {
    globalLoading.value = true
    const enabledTargets = targets.value.filter(t => t.enabled && t.datasourceId && t.expression.trim())
    await Promise.allSettled(enabledTargets.map(executeQuery))
    globalLoading.value = false
  }

  return {
    targets,
    globalLoading,
    addTarget,
    removeTarget,
    toggleTarget,
    updateTarget,
    executeAll,
    executeQuery,
  }
}
