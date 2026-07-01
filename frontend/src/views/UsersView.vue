<template>
  <section class="page">
    <PageHeader title="用户角色">
      <template #actions>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增用户</el-button>
      </template>
    </PageHeader>
    <div class="panel">
      <el-table :data="items" v-loading="loading">
        <el-table-column prop="username" label="账号" width="150" />
        <el-table-column prop="name" label="姓名" min-width="160" />
        <el-table-column prop="authSource" label="来源" width="100">
          <template #default="{ row }">
            <StatusTag :value="row.authSource" :map="AUTH_SOURCE_MAP" />
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="department" label="部门" width="140" />
        <el-table-column prop="role" label="角色" width="160">
          <template #default="{ row }">
            <RoleTag :value="row.role" />
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <StatusTag :value="row.status" :map="USER_STATUS_MAP" />
          </template>
        </el-table-column>
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
            <el-option v-for="option in roleOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态"><el-select v-model="form.status"><el-option v-for="option in userStatusOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item label="部门"><el-input v-model="form.department" /></el-form-item>
        <el-form-item v-if="form.authSource !== 'ad'" label="密码"><el-input v-model="form.password" type="password" show-password placeholder="编辑时留空则不修改" /></el-form-item>
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
import { usersApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'
import RoleTag from '../components/common/RoleTag.vue'
import StatusTag from '../components/common/StatusTag.vue'
import { AUTH_SOURCE_MAP, ROLE_MAP, USER_STATUS_MAP, dictOptions } from '../constants/dictionaries'

const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive(emptyUser())
const roleOptions = dictOptions(ROLE_MAP)
const userStatusOptions = dictOptions(USER_STATUS_MAP)

function emptyUser() {
  return { username: '', name: '', role: 'applicant', status: 'active', password: '', authSource: 'local', email: '', department: '' }
}

async function load() {
  loading.value = true
  try {
    const data = await usersApi.list()
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
  if (form.id) await usersApi.update(form.id, form)
  else await usersApi.create(form)
  ElMessage.success('已保存')
  dialogVisible.value = false
  load()
}

onMounted(load)
</script>
