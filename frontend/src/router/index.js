import { createRouter, createWebHistory } from 'vue-router'
import axios from 'axios'

const routes = [
  { path: '/setup', component: () => import('../views/Setup.vue'), meta: { guest: true, setup: true } },
  { path: '/login', component: () => import('../views/Login.vue'), meta: { guest: true } },
  {
    path: '/',
    component: () => import('../components/Layout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', component: () => import('../views/Dashboard.vue') },
      { path: 'subscriptions', component: () => import('../views/Subscriptions.vue') },
      { path: 'calendar', component: () => import('../views/Calendar.vue') },
      { path: 'reports', component: () => import('../views/Reports.vue') },
      { path: 'notifications', component: () => import('../views/Notifications.vue') },
      { path: 'logs', component: () => import('../views/Logs.vue') },
      { path: 'settings', component: () => import('../views/Settings.vue') },
      { path: 'users', component: () => import('../views/Users.vue') }
    ]
  }
]

const router = createRouter({ history: createWebHistory(), routes })

// Embedded SQLite is initialized by the server at startup. Keep a lightweight
// readiness check so navigation waits for migrations on a cold start.
let configured = null
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

async function checkConfigured() {
  if (configured === true) return true
  for (let i = 0; i < 3; i++) {
    try {
      const { data } = await axios.get('/api/setup/status')
      if (data.configured) { configured = true; return true }
    } catch {
      // The process may still be opening the local database.
    }
    await sleep(600)
  }
  return false
}

router.beforeEach(async (to) => {
  const isConfigured = await checkConfigured()

  // 未配置数据库：强制进入安装向导
  if (!isConfigured) {
    return to.meta.setup ? true : '/setup'
  }
  // 已配置：不再允许访问安装向导
  if (to.meta.setup) return '/login'

  // 登录态校验
  const loggedIn = !!localStorage.getItem('access_token')
  if (!to.meta.guest && !loggedIn) return '/login'
  if (to.meta.guest && loggedIn) return '/dashboard'
})

export default router
