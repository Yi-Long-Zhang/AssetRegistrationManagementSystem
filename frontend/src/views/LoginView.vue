<template>
  <main class="login">
    <div class="glow glow-a"></div>
    <div class="glow glow-b"></div>
    <div class="glow glow-c"></div>
    <section class="login-card">
      <div class="card-brand">
        <span class="brand-mark">资</span>
      </div>
      <h1>资产管理系统</h1>
      <p class="subtitle">服务器资产 · 工单流程 · 动态发现</p>
      <el-form :model="form" label-position="top" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="form.username" autocomplete="username" placeholder="请输入账号" :prefix-icon="User" size="large" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            type="password"
            autocomplete="current-password"
            show-password
            placeholder="请输入密码"
            :prefix-icon="Lock"
            size="large"
            @keyup.enter="submit"
          />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" class="submit-btn" size="large">登 录</el-button>
      </el-form>
      <p class="muted hint">支持本地账号或已导入的域账号登录</p>
    </section>
  </main>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Lock, User } from '@element-plus/icons-vue'
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
  position: relative;
  min-height: 100vh;
  display: grid;
  place-items: center;
  overflow: hidden;
  background:
    radial-gradient(900px 500px at 80% 10%, rgba(99, 102, 241, 0.12), transparent 60%),
    radial-gradient(800px 500px at 10% 90%, rgba(168, 85, 247, 0.1), transparent 60%),
    linear-gradient(135deg, #eef2ff 0%, #f5f3ff 50%, #f8fafc 100%);
}

/* 漂浮光晕 */
.glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.5;
  animation: float 9s ease-in-out infinite;
}

.glow-a {
  width: 320px;
  height: 320px;
  background: rgba(99, 102, 241, 0.35);
  top: -80px;
  right: 12%;
}

.glow-b {
  width: 260px;
  height: 260px;
  background: rgba(168, 85, 247, 0.32);
  bottom: -60px;
  left: 10%;
  animation-delay: -3s;
}

.glow-c {
  width: 180px;
  height: 180px;
  background: rgba(14, 165, 233, 0.28);
  top: 40%;
  left: 55%;
  animation-delay: -6s;
}

@keyframes float {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
  }
  50% {
    transform: translate(30px, -24px) scale(1.08);
  }
}

.login-card {
  position: relative;
  z-index: 1;
  width: min(420px, calc(100vw - 32px));
  background: rgba(255, 255, 255, 0.88);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.9);
  border-radius: 24px;
  padding: 36px 32px 28px;
  box-shadow: 0 24px 70px rgba(79, 70, 229, 0.16);
  animation: card-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
}

@keyframes card-in {
  from {
    opacity: 0;
    transform: translateY(24px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.card-brand {
  display: flex;
  justify-content: center;
  margin-bottom: 14px;
}

.brand-mark {
  width: 56px;
  height: 56px;
  display: grid;
  place-items: center;
  border-radius: 18px;
  background: var(--brand-gradient);
  color: #fff;
  font-size: 26px;
  font-weight: 700;
  box-shadow: 0 10px 26px rgba(99, 102, 241, 0.45);
}

h1 {
  margin: 0 0 6px;
  text-align: center;
  font-size: 24px;
  font-weight: 700;
  background: linear-gradient(90deg, #4f46e5, #7c3aed);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.subtitle {
  margin: 0 0 26px;
  text-align: center;
  color: var(--text-secondary);
  font-size: 13px;
  letter-spacing: 0.5px;
}

.submit-btn {
  width: 100%;
  height: 44px;
  font-size: 15px;
  letter-spacing: 4px;
}

.hint {
  margin: 18px 0 0;
  text-align: center;
  font-size: 12px;
}
</style>
