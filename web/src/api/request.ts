import axios from 'axios'
import type { ApiResponse } from '@/types'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

// Map backend error codes to user-friendly messages
// Key is the error code, values are [zh, en]
const errorMessages: Record<number, [string, string]> = {
  10102: ['用户名或密码错误', 'Invalid username or password'],
  10101: ['登录已过期，请重新登录', 'Session expired, please log in again'],
  10100: ['未授权访问', 'Unauthorized'],
  10200: ['权限不足', 'Insufficient permissions'],
  10300: ['资源不存在', 'Resource not found'],
  10400: ['名称已被占用', 'Name already taken'],
}

function getLocale(): string {
  try { return localStorage.getItem('locale') || 'zh-CN' } catch { return 'zh-CN' }
}

function localizeError(code: number, fallback: string): string {
  const msgs = errorMessages[code]
  if (!msgs) return fallback
  return getLocale() === 'zh-CN' ? msgs[0] : msgs[1]
}

// Request interceptor - attach JWT token
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) config.headers.Authorization = `Bearer ${token}`
    return config
  },
  (error) => Promise.reject(error)
)

// Prevent multiple simultaneous 401 redirects / refresh attempts
let isRedirecting = false
let refreshPromise: Promise<string> | null = null

function redirectToLogin() {
  if (isRedirecting) return
  isRedirecting = true
  localStorage.removeItem('token')
  localStorage.removeItem('user_role')
  import('@/router').then(({ default: router }) => {
    router.push({ name: 'Login', query: { redirect: router.currentRoute.value.fullPath } })
  }).finally(() => {
    setTimeout(() => { isRedirecting = false }, 2000)
  })
}

// Response interceptor — auto-refresh token on 401 before giving up
request.interceptors.response.use(
  (response) => {
    const data = response.data as ApiResponse
    if (data.code !== 0) {
      const msg = localizeError(data.code, data.message || 'Unknown error')
      return Promise.reject(new Error(msg))
    }
    return response
  },
  async (error) => {
    const originalRequest = error.config
    if (error.response?.status === 401 && !originalRequest._retried) {
      originalRequest._retried = true
      const storedToken = localStorage.getItem('token')
      if (storedToken && !isRedirecting) {
        try {
          // Deduplicate concurrent refresh calls
          if (!refreshPromise) {
            refreshPromise = (async () => {
              const res = await axios.post('/api/v1/auth/refresh', { token: storedToken })
              const newToken: string = res.data?.data?.token
              if (!newToken) throw new Error('empty token')
              return newToken
            })().finally(() => { refreshPromise = null })
          }
          const newToken = await refreshPromise
          localStorage.setItem('token', newToken)
          originalRequest.headers.Authorization = `Bearer ${newToken}`
          return request(originalRequest)
        } catch {
          redirectToLogin()
          return Promise.reject(error)
        }
      }
      redirectToLogin()
      return Promise.reject(error)
    }
    const data = error.response?.data as ApiResponse | undefined
    const code = data?.code || 0
    const fallback = data?.message || error.message || 'Network error'
    return Promise.reject(new Error(localizeError(code, fallback)))
  }
)

export default request
