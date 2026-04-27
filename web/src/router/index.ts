import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/Login.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: '/dashboard',
      },
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/pages/dashboard/Index.vue'),
        meta: { title: 'Dashboard', icon: 'dashboard' },
      },
      {
        path: 'datasources',
        name: 'DataSources',
        component: () => import('@/pages/datasources/Index.vue'),
        meta: { title: 'Data Sources', icon: 'server' },
      },
      {
        path: 'datasources/query',
        name: 'DatasourceQuery',
        component: () => import('@/pages/datasources/Query.vue'),
        meta: { title: 'Query', icon: 'search' },
      },
      {
        path: 'alerts',
        name: 'Alerts',
        redirect: '/alerts/rules',
        children: [
          {
            path: 'rules',
            name: 'AlertRules',
            component: () => import('@/pages/alerts/rules/Index.vue'),
            meta: { title: 'Alert Rules', icon: 'rule' },
          },
          {
            path: 'events',
            name: 'AlertEvents',
            component: () => import('@/pages/alerts/events/Index.vue'),
            meta: { title: 'Active Alerts', icon: 'alert' },
          },
          {
            path: 'events/:id',
            name: 'AlertEventDetail',
            component: () => import('@/pages/alerts/events/Detail.vue'),
            meta: { title: 'Alert Detail' },
          },
          {
            path: 'history',
            name: 'AlertHistory',
            component: () => import('@/pages/alerts/history/Index.vue'),
            meta: { title: 'Alert History', icon: 'history' },
          },
          {
            path: 'mute-rules',
            name: 'MuteRules',
            component: () => import('@/pages/alerts/mute/Index.vue'),
            meta: { title: 'Mute Rules', icon: 'mute' },
          },
          {
            path: 'inhibition-rules',
            name: 'InhibitionRules',
            component: () => import('@/pages/alerts/inhibition/Index.vue'),
            meta: { title: 'Inhibition Rules', icon: 'inhibition' },
          },
        ],
      },
      {
        path: 'notification',
        name: 'Notification',
        component: () => import('@/pages/notification/Index.vue'),
        meta: { title: 'Notification' },
      },
      {
        path: 'schedule',
        name: 'Schedule',
        component: () => import('@/pages/schedule/Index.vue'),
        meta: { title: 'On-Call Schedule', icon: 'calendar' },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/pages/settings/Index.vue'),
        meta: { title: 'Settings', icon: 'settings', requiresRole: ['admin', 'team_lead'] },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Navigation guard
router.beforeEach((to, _from, next) => {
  // Handle OIDC callback: extract token from URL hash fragment
  // Backend redirects to /#oidc_token=...&expires_in=...
  const hash = window.location.hash
  if (hash && hash.includes('oidc_token=')) {
    const params = new URLSearchParams(hash.substring(1)) // strip leading #
    const oidcToken = params.get('oidc_token')
    if (oidcToken) {
      localStorage.setItem('token', oidcToken)
      // Clear the hash fragment
      window.history.replaceState(null, '', '/')
      next({ name: 'Dashboard', replace: true })
      return
    }
  }

  // Also handle query param for backwards compatibility
  const oidcTokenQuery = to.query.oidc_token as string | undefined
  if (oidcTokenQuery) {
    localStorage.setItem('token', oidcTokenQuery)
    next({ name: 'Dashboard', replace: true })
    return
  }

  const token = localStorage.getItem('token')

  if (to.meta.requiresAuth !== false && !token) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
  } else if (to.name === 'Login' && token) {
    next({ name: 'Dashboard' })
  } else if (to.meta.requiresRole) {
    // Route-level role guard
    const userStr = localStorage.getItem('user_role')
    const allowedRoles = to.meta.requiresRole as string[]
    if (userStr && !allowedRoles.includes(userStr)) {
      next({ name: 'Dashboard' })
    } else {
      next()
    }
  } else {
    next()
  }
})

export default router
