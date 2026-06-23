<template>
  <section class="page">
    <div class="toolbar">
      <h2>服务器资产</h2>
      <div class="toolbar-actions">
        <el-input v-model="keyword" placeholder="搜索资产编号、主机名、IP" clearable @keyup.enter="load" />
        <el-button :icon="Search" @click="load" />
        <el-button v-if="canManage" type="primary" :icon="Plus" @click="openCreate">新增资产</el-button>
      </div>
    </div>
    <div class="panel">
      <el-table :data="items" v-loading="loading">
        <el-table-column prop="assetNo" label="资产编号" width="140" />
        <el-table-column prop="hostname" label="主机名" min-width="160" />
        <el-table-column prop="ip" label="IP" width="150" />
        <el-table-column prop="location" label="机房" width="140" />
        <el-table-column prop="owner" label="负责人" width="120" />
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column v-if="canManage" label="操作" width="160">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑资产' : '新增资产'" width="720px">
      <el-form :model="form" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="资产编号"><el-input v-model="form.assetNo" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="主机名"><el-input v-model="form.hostname" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="IP"><el-input v-model="form.ip" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="状态"><el-select v-model="form.status"><el-option label="使用中" value="in_use" /><el-option label="维护中" value="maintenance" /><el-option label="已退役" value="retired" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="机房"><el-input v-model="form.location" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="机柜"><el-input v-model="form.rack" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="系统"><el-input v-model="form.os" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="负责人"><el-input v-model="form.owner" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="CPU"><el-input v-model="form.cpu" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="内存"><el-input v-model="form.memory" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="磁盘"><el-input v-model="form.disk" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" /></el-form-item>
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import { api } from '../api'

const user = JSON.parse(localStorage.getItem('user') || 'null')
const canManage = computed(() => ['admin', 'asset_manager'].includes(user?.role))
const items = ref([])
const loading = ref(false)
const keyword = ref('')
const dialogVisible = ref(false)
const form = reactive(emptyAsset())

function emptyAsset() {
  return { assetNo: '', hostname: '', ip: '', status: 'in_use', location: '', rack: '', os: '', cpu: '', memory: '', disk: '', owner: '', remark: '' }
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/assets', { params: { q: keyword.value } })
    items.value = data.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, emptyAsset(), { id: undefined })
  dialogVisible.value = true
}

function openEdit(row) {
  Object.assign(form, row)
  dialogVisible.value = true
}

async function save() {
  try {
    if (form.id) await api.put(`/assets/${form.id}`, form)
    else await api.post('/assets', form)
    ElMessage.success('已保存')
    dialogVisible.value = false
    load()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存失败')
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除资产 ${row.assetNo}？`, '删除确认', { type: 'warning' })
  await api.delete(`/assets/${row.id}`)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
