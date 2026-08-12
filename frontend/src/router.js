import { createRouter, createWebHistory } from 'vue-router'
import { ElMessage } from 'element-plus'
import LoginView from './views/LoginView.vue'
import DashboardView from './views/DashboardView.vue'
import AssetsView from './views/AssetsView.vue'
import AuditLogsView from './views/AuditLogsView.vue'
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
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', component: DashboardView, meta: { auth: true, menu: true, title: '数据看板', icon: 'Odometer', order: 5 } },
        { path: 'assets', component: AssetsView, meta: { auth: true, menu: true, title: '服务器资产', icon: 'Monitor', order: 10 } },
        { path: 'discovery', component: () => import('./views/DiscoveryView.vue'), meta: { auth: true, menu: true, title: '资产发现', icon: 'Aim', roles: ['admin', 'asset_manager'], order: 15 } },
        { path: 'tickets', component: TicketsView, meta: { auth: true, menu: true, title: '工单流程', icon: 'Tickets', order: 20 } },
        { path: 'ticket-stats', component: () => import('./views/TicketsStatsView.vue'), meta: { auth: true, menu: true, title: '工单报表', icon: 'TrendCharts', roles: ['admin'], order: 22 } },
        { path: 'inspection', component: () => import('./views/InspectionRulesView.vue'), meta: { auth: true, menu: true, title: '巡检规则', icon: 'AlarmClock', roles: ['admin'], order: 25 } },
        { path: 'workflows', component: WorkflowsView, meta: { auth: true, menu: true, title: '流程配置', icon: 'Operation', roles: ['admin'], order: 30 } },
        { path: 'users', component: UsersView, meta: { auth: true, menu: true, title: '用户管理', icon: 'User', roles: ['admin'], order: 40 } },
        { path: 'audit-logs', component: AuditLogsView, meta: { auth: true, menu: true, title: '操作审计', icon: 'Document', roles: ['admin'], order: 45 } },
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

  if (to.path === '/login' && auth.isLoggedIn) return '/dashboard'
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
