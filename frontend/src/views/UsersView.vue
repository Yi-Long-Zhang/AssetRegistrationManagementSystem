<template>
  <section class="page">
    <div class="toolbar">
      <h2>用户角色</h2>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增用户</el-button>
    </div>
    <div class="panel">
      <el-table :data="items" v-loading="loading">
        <el-table-column prop="username" label="账号" width="150" />
        <el-table-column prop="name" label="姓名" min-width="160" />
        <el-table-column prop="role" label="角色" width="160" />
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑用户' : '新增用户'" width="520px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="账号"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role">
            <el-option label="管理员" value="admin" />
            <el-option label="资产管理员" value="asset_manager" />
            <el-option label="审批人" value="approver" />
            <el-option label="申请人" value="applicant" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态"><el-select v-model="form.status"><el-option label="启用" value="active" /><el-option label="禁用" value="disabled" /></el-select></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" type="password" show-password placeholder="编辑时留空则不修改" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { api } from '../api'

const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive(emptyUser())

function emptyUser() {
  return { username: '', name: '', role: 'applicant', status: 'active', password: '' }
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/users')
    items.value = data.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, emptyUser(), { id: undefined })
  dialogVisible.value = true
}

function openEdit(row) {
  Object.assign(form, row, { password: '' })
  dialogVisible.value = true
}

async function save() {
  if (form.id) await api.put(`/users/${form.id}`, form)
  else await api.post('/users', form)
  ElMessage.success('已保存')
  dialogVisible.value = false
  load()
}

onMounted(load)
</script>
