<template>
  <section class="page">
    <PageHeader title="服务器资产">
      <template #actions>
        <el-button v-if="canManage" type="primary" :icon="Plus" @click="openCreate">新增资产</el-button>
      </template>
    </PageHeader>
    <DataToolbar>
      <template #actions>
        <el-input v-model="keyword" placeholder="搜索资产编号、主机名、IP" clearable @keyup.enter="load" />
        <el-button :icon="Search" @click="load" />
      </template>
    </DataToolbar>
    <div class="panel">
      <el-table :data="items" v-loading="loading">
        <el-table-column prop="assetNo" label="资产编号" width="140" />
        <el-table-column prop="hostname" label="主机名" min-width="160" />
        <el-table-column prop="ip" label="IP" width="150" />
        <el-table-column prop="location" label="机房" width="140" />
        <el-table-column prop="owner" label="负责人" width="120" />
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <StatusTag :value="row.status" :map="ASSET_STATUS_MAP" />
          </template>
        </el-table-column>
        <el-table-column v-if="canManage" label="操作" width="160">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <ConfirmAction link type="danger" :message="`确认删除资产 ${row.assetNo}？`" @confirm="remove(row)">删除</ConfirmAction>
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
import { ElMessage } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import { assetsApi } from '../api'
import ConfirmAction from '../components/common/ConfirmAction.vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import PageHeader from '../components/common/PageHeader.vue'
import StatusTag from '../components/common/StatusTag.vue'
import { ASSET_STATUS_MAP } from '../constants/dictionaries'
import { useAuthStore } from '../stores/auth'
import { canManageAssets } from '../utils/permissions'

const auth = useAuthStore()
const canManage = computed(() => canManageAssets(auth.user))
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
    const data = await assetsApi.list({ q: keyword.value })
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
    if (form.id) await assetsApi.update(form.id, form)
    else await assetsApi.create(form)
    ElMessage.success('已保存')
    dialogVisible.value = false
    load()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存失败')
  }
}

async function remove(row) {
  await assetsApi.remove(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
