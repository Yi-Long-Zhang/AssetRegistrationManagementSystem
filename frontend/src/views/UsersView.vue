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

    <div class="panel approver-panel">
      <div class="toolbar">
        <h3>工单类型审批人配置</h3>
      </div>
      <el-table :data="ticketTypes">
        <el-table-column prop="label" label="工单类型" min-width="180" />
        <el-table-column label="默认审批人" min-width="220">
          <template #default="{ row }">
            <el-select v-model="approverMap[row.value]" placeholder="选择审批人">
              <el-option v-for="user in approverUsers" :key="user.id" :label="`${user.name} (${user.username})`" :value="user.id" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button type="primary" link @click="saveApprover(row.value)">保存</el-button>
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
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { api } from '../api'

const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive(emptyUser())
const approverMap = reactive({})
const ticketTypes = [
  { label: '资产登记', value: 'asset_register' },
  { label: '资产变更', value: 'asset_change' },
  { label: '资产下线/报废', value: 'asset_retire' },
  { label: '权限/维护申请', value: 'maintenance' }
]
const approverUsers = computed(() => items.value.filter((user) => ['admin', 'approver'].includes(user.role) && user.status === 'active'))

function emptyUser() {
  return { username: '', name: '', role: 'applicant', status: 'active', password: '' }
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/users')
    items.value = data.items
    await loadApprovers()
  } finally {
    loading.value = false
  }
}

async function loadApprovers() {
  const { data } = await api.get('/ticket-type-approvers')
  for (const item of data.items) {
    approverMap[item.type] = item.approverId
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

async function saveApprover(type) {
  if (!approverMap[type]) {
    ElMessage.warning('请选择审批人')
    return
  }
  await api.put(`/ticket-type-approvers/${type}`, { approverId: approverMap[type] })
  ElMessage.success('审批人配置已保存')
  await loadApprovers()
}

onMounted(load)
</script>

<style scoped>
.approver-panel {
  margin-top: 16px;
}
</style>
