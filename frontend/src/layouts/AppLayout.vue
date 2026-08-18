<template>
  <el-container class="layout">
    <el-aside :width="app.sidebarCollapsed ? '64px' : '220px'" class="aside">
      <div class="brand" :class="{ collapsed: app.sidebarCollapsed }">
        <span class="brand-mark">资</span>
        <span v-if="!app.sidebarCollapsed" class="brand-text">资产管理</span>
      </div>
      <el-menu
        router
        :collapse="app.sidebarCollapsed"
        :default-active="$route.path"
        class="side-menu"
      >
        <el-menu-item v-for="item in menus" :key="item.path" :index="item.path">
          <el-icon><component :is="iconMap[item.meta.icon] || Menu" /></el-icon>
          <template #title>{{ item.meta.title }}</template>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-button text class="collapse-btn" :icon="app.sidebarCollapsed ? Expand : Fold" @click="app.toggleSidebar" />
          <span class="header-title">{{ app.pageTitle }}</span>
        </div>
        <el-dropdown trigger="click" @command="handleCommand">
          <button class="user-button">
            <span class="user-avatar">{{ (auth.user?.name || auth.user?.username || '?').slice(0, 1) }}</span>
            <span class="user-name">{{ auth.user?.name || auth.user?.username }}</span>
            <RoleTag :value="auth.user?.role" />
            <el-icon class="chevron"><ArrowDown /></el-icon>
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item disabled>{{ auth.user?.username }}</el-dropdown-item>
              <el-dropdown-item divided command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main>
        <router-view v-slot="{ Component }">
          <transition name="route-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Aim, ArrowDown, Connection, Expand, Fold, Menu, Monitor, Operation, Setting, Stamp, Tickets, Timer, User } from '@element-plus/icons-vue'
import RoleTag from '../components/common/RoleTag.vue'
import { useAppStore } from '../stores/app'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const app = useAppStore()
const auth = useAuthStore()

const iconMap = { Aim, Connection, Monitor, Operation, Setting, Stamp, Tickets, Timer, User }
const menus = computed(() =>
  router
    .getRoutes()
    .filter((route) => route.meta?.menu && !route.meta?.hidden && auth.canAccess(route.meta.roles))
    .sort((a, b) => (a.meta.order || 0) - (b.meta.order || 0))
)

async function handleCommand(command) {
  if (command === 'logout') {
    await auth.logout()
    router.push('/login')
  }
}
</script>

<style scoped>
.layout {
  min-height: 100vh;
}

/* ---------- 侧边栏 ---------- */
.aside {
  background:
    radial-gradient(400px 300px at 0% 0%, rgba(99, 102, 241, 0.25), transparent 70%),
    #0f1420;
  transition: width var(--dur-base) var(--ease-out);
  overflow-x: hidden;
}

.brand {
  height: 60px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  color: #fff;
  font-weight: 700;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  white-space: nowrap;
}

.brand.collapsed {
  justify-content: center;
  padding: 0;
}

.brand-mark {
  width: 32px;
  height: 32px;
  display: inline-grid;
  place-items: center;
  border-radius: 10px;
  background: var(--brand-gradient);
  color: #fff;
  font-size: 16px;
  box-shadow: 0 4px 14px rgba(99, 102, 241, 0.45);
  flex-shrink: 0;
}

.brand-text {
  background: linear-gradient(90deg, #e0e7ff, #c4b5fd);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  font-size: 16px;
  letter-spacing: 0.5px;
}

.side-menu {
  border-right: none;
  background: transparent;
  padding: 10px 8px;
  --el-menu-bg-color: transparent;
  --el-menu-text-color: #9ca3af;
  --el-menu-hover-bg-color: rgba(255, 255, 255, 0.06);
  --el-menu-active-color: #ffffff;
}

.side-menu :deep(.el-menu-item) {
  height: 44px;
  line-height: 44px;
  border-radius: 10px;
  margin-bottom: 4px;
  transition: background-color var(--dur-fast) var(--ease-standard), transform var(--dur-fast) var(--ease-out), color var(--dur-fast) var(--ease-standard);
}

.side-menu :deep(.el-menu-item:hover) {
  transform: translateX(3px);
}

.side-menu :deep(.el-menu-item.is-active) {
  background: var(--brand-gradient);
  box-shadow: 0 4px 14px rgba(99, 102, 241, 0.4);
  transform: translateX(3px);
}

.side-menu :deep(.el-menu--collapse .el-menu-item) {
  border-radius: 10px;
}

/* ---------- 顶栏（毛玻璃） ---------- */
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: rgba(255, 255, 255, 0.75);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(229, 231, 235, 0.7);
  height: 60px;
  padding: 0 20px;
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.collapse-btn {
  color: var(--text-secondary);
  border-radius: 8px;
}

.collapse-btn:hover {
  background: rgba(99, 102, 241, 0.08);
}

.header-title {
  font-weight: 700;
  font-size: 15px;
  background: linear-gradient(90deg, #4f46e5, #7c3aed);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.user-button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border: none;
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  border-radius: 10px;
  transition: background-color 0.2s ease;
}

.user-button:hover {
  background: rgba(99, 102, 241, 0.08);
}

.user-avatar {
  width: 30px;
  height: 30px;
  display: inline-grid;
  place-items: center;
  border-radius: 50%;
  background: var(--brand-gradient);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  box-shadow: 0 2px 8px rgba(99, 102, 241, 0.4);
}

.user-name {
  font-weight: 600;
}

.chevron {
  color: var(--text-secondary);
  font-size: 12px;
}

/* ---------- 页面切换过渡 ---------- */
.route-fade-enter-active {
  transition: opacity var(--dur-slow) var(--ease-out), transform var(--dur-slow) var(--ease-out);
}

.route-fade-leave-active {
  transition: opacity var(--dur-fast) var(--ease-standard), transform var(--dur-fast) var(--ease-standard);
}

.route-fade-enter-from {
  opacity: 0;
  transform: translateY(12px) scale(0.995);
}

.route-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px) scale(0.998);
}

.el-main {
  background: transparent;
  padding: 0;
}

.el-main > * {
  padding: 24px;
}
</style>
