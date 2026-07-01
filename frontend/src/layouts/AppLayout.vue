<template>
  <el-container class="layout">
    <el-aside :width="app.sidebarCollapsed ? '64px' : '220px'" class="aside">
      <div class="brand" :class="{ collapsed: app.sidebarCollapsed }">
        <span class="brand-mark">资</span>
        <span v-if="!app.sidebarCollapsed">资产管理</span>
      </div>
      <el-menu
        router
        :collapse="app.sidebarCollapsed"
        :default-active="$route.path"
        background-color="#111827"
        text-color="#d1d5db"
        active-text-color="#ffffff"
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
          <el-button text :icon="app.sidebarCollapsed ? Expand : Fold" @click="app.toggleSidebar" />
          <span class="header-title">{{ app.pageTitle }}</span>
        </div>
        <el-dropdown trigger="click" @command="handleCommand">
          <button class="user-button">
            <span>{{ auth.user?.name || auth.user?.username }}</span>
            <RoleTag :value="auth.user?.role" />
            <el-icon><ArrowDown /></el-icon>
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
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowDown, Expand, Fold, Menu, Monitor, Operation, Setting, Tickets, User } from '@element-plus/icons-vue'
import RoleTag from '../components/common/RoleTag.vue'
import { useAppStore } from '../stores/app'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const app = useAppStore()
const auth = useAuthStore()

const iconMap = { Monitor, Operation, Setting, Tickets, User }
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

.aside {
  background: #111827;
  transition: width 0.2s ease;
}

.brand {
  height: 56px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 18px;
  color: #fff;
  font-weight: 700;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  white-space: nowrap;
}

.brand.collapsed {
  justify-content: center;
  padding: 0;
}

.brand-mark {
  width: 28px;
  height: 28px;
  display: inline-grid;
  place-items: center;
  border-radius: 6px;
  background: #2563eb;
  color: #fff;
  font-size: 15px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.header-title {
  font-weight: 600;
  color: #111827;
}

.user-button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0;
  border: none;
  background: transparent;
  color: #1f2937;
  cursor: pointer;
  font: inherit;
}

.el-menu {
  border-right: none;
}
</style>
