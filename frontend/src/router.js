import { createRouter, createWebHistory } from 'vue-router'
import LoginView from './views/LoginView.vue'
import DashboardView from './views/DashboardView.vue'
import AssetsView from './views/AssetsView.vue'
import TicketsView from './views/TicketsView.vue'
import UsersView from './views/UsersView.vue'

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
        { path: 'assets', component: AssetsView },
        { path: 'tickets', component: TicketsView },
        { path: 'users', component: UsersView, meta: { roles: ['admin'] } }
      ]
    }
  ]
})

router.beforeEach((to) => {
  const token = localStorage.getItem('token')
  const user = JSON.parse(localStorage.getItem('user') || 'null')
  if (to.meta.auth && !token) return '/login'
  if (to.meta.roles && !to.meta.roles.includes(user?.role)) return '/assets'
})

export default router
