<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="brand">资产管理</div>
      <el-menu router :default-active="$route.path" background-color="#111827" text-color="#d1d5db" active-text-color="#ffffff">
        <el-menu-item index="/assets"><el-icon><Monitor /></el-icon><span>服务器资产</span></el-menu-item>
        <el-menu-item index="/tickets"><el-icon><Tickets /></el-icon><span>工单流程</span></el-menu-item>
        <el-menu-item v-if="user?.role === 'admin'" index="/users"><el-icon><User /></el-icon><span>用户角色</span></el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span>{{ user?.name }} · {{ roleLabel(user?.role) }}</span>
        <el-button @click="logout">退出</el-button>
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
import { Monitor, Tickets, User } from '@element-plus/icons-vue'

const router = useRouter()
const user = computed(() => JSON.parse(localStorage.getItem('user') || 'null'))

function roleLabel(role) {
  return ({ admin: '管理员', asset_manager: '资产管理员', approver: '审批人', applicant: '申请人' })[role] || role
}

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  router.push('/login')
}
</script>

<style scoped>
.layout {
  min-height: 100vh;
}

.aside {
  background: #111827;
}

.brand {
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  color: #fff;
  font-weight: 700;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}

.el-menu {
  border-right: none;
}
</style>
