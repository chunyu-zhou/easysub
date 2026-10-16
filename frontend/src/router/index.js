import { createRouter, createWebHistory } from 'vue-router'

const routes = [
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

router.beforeEach(async (to) => {
  // Embedded SQLite is created, migrated, and seeded automatically at startup.
  // There is no database setup step in the browser.

  // 登录态校验
  const loggedIn = !!localStorage.getItem('access_token')
  if (!to.meta.guest && !loggedIn) return '/login'
  if (to.meta.guest && loggedIn) return '/dashboard'
})

export default router
