/** Time range for queries (Unix milliseconds) */
export interface TimeRange {
  start: number
  end: number
}

/** A single query target in the Explore panel */
export interface QueryTarget {
  id: string
  datasourceId: number | null
  expression: string
  legendFormat: string
  enabled: boolean
  // Query result state
  state: 'idle' | 'loading' | 'error'
  resultType: 'vector' | 'matrix' | null
  series: QuerySeriesItem[]
  error: string | null
}

/** A single time series returned by a query */
export interface QuerySeriesItem {
  labels: Record<string, string>
  values: Array<{ ts: number; value: number }>
}

/** Request body for range query API */
export interface RangeQueryRequest {
  expression: string
  start: number    // Unix seconds
  end: number      // Unix seconds
  step: string     // e.g. "15s", "1m", "5m"
}

/** Auto-refresh option */
export interface AutoRefreshOption {
  label: string
  value: number | null  // ms, null = off
}

/** Relative time option */
export interface RelativeTimeOption {
  label: string
  value: string        // e.g. "1h", "6h", "7d"
  ms: number
}

/** A single log entry returned by VictoriaLogs */
export interface LogEntry {
  timestamp: string
  message: string
  labels: Record<string, any>
}

/** Response from log query API */
export interface LogQueryResponse {
  entries: LogEntry[]
  total: number
  truncated: boolean
}
