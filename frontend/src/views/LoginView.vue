<template>
  <main class="login">
    <section class="login-panel">
      <h1>资产管理系统</h1>
      <el-form :model="form" label-position="top" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="form.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" autocomplete="current-password" show-password />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading">登录</el-button>
      </el-form>
      <p class="muted">支持本地账号或已导入的域账号登录</p>
    </section>
  </main>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const form = reactive({ username: 'admin', password: 'admin123456' })

async function submit() {
  loading.value = true
  try {
    await auth.login(form)
    router.push('/assets')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #eef2ff 0%, #ecfeff 45%, #f8fafc 100%);
}

.login-panel {
  width: min(420px, calc(100vw - 32px));
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 28px;
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.12);
}

h1 {
  margin: 0 0 24px;
  font-size: 24px;
}

.el-button {
  width: 100%;
}
</style>
