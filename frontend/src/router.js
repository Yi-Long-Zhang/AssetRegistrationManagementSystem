import { createRouter, createWebHistory } from 'vue-router'
import { ElMessage } from 'element-plus'
import LoginView from './views/LoginView.vue'
import DashboardView from './views/DashboardView.vue'
import AssetsView from './views/AssetsView.vue'
import TicketsView from './views/TicketsView.vue'
import UsersView from './views/UsersView.vue'
import WorkflowsView from './views/WorkflowsView.vue'
import SettingsView from './views/SettingsView.vue'
import { useAppStore } from './stores/app'
import { useAuthStore } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView },
    {
      path: '/',
      component: DashboardView,
      meta: { auth: true },
      children: [
        { path: '', redirect: '/assets' },
        { path: 'assets', component: AssetsView, meta: { auth: true, menu: true, title: '服务器资产', icon: 'Monitor', order: 10 } },
        { path: 'tickets', component: TicketsView, meta: { auth: true, menu: true, title: '工单流程', icon: 'Tickets', order: 20 } },
        { path: 'workflows', component: WorkflowsView, meta: { auth: true, menu: true, title: '流程配置', icon: 'Operation', roles: ['admin'], order: 30 } },
        { path: 'users', component: UsersView, meta: { auth: true, menu: true, title: '用户管理', icon: 'User', roles: ['admin'], order: 40 } },
        { path: 'settings', component: SettingsView, meta: { auth: true, menu: true, title: '系统配置', icon: 'Setting', roles: ['admin'], order: 50 } }
      ]
    }
  ]
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  const app = useAppStore()
  const needAuth = to.matched.some((record) => record.meta.auth)
  const roles = to.matched.flatMap((record) => record.meta.roles || [])

  if (to.path === '/login' && auth.isLoggedIn) return '/assets'
  if (needAuth && !auth.isLoggedIn) return '/login'

  if (needAuth && auth.isLoggedIn && !auth.loaded) {
    try {
      await auth.fetchMe()
    } catch {
      auth.clearSession()
      return '/login'
    }
  }

  if (roles.length && !auth.canAccess(roles)) {
    ElMessage.warning('无权限访问该页面')
    return '/assets'
  }

  app.setPageTitle(to.meta.title)
})

export default router
