<template>
  <section class="page">
    <div class="toolbar">
      <h2>工单流程</h2>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建工单</el-button>
    </div>
    <div class="panel">
      <el-table :data="items" v-loading="loading">
        <el-table-column prop="id" label="编号" width="80" />
        <el-table-column prop="title" label="标题" min-width="180" />
        <el-table-column prop="type" label="类型" width="150" />
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column prop="priority" label="优先级" width="100" />
        <el-table-column label="申请人" width="120"><template #default="{ row }">{{ row.applicant?.name }}</template></el-table-column>
        <el-table-column label="操作" width="360">
          <template #default="{ row }">
            <el-button link type="primary" @click="view(row)">详情</el-button>
            <el-button v-for="action in actions(row)" :key="action.name" link type="primary" @click="doAction(row, action.name)">{{ action.label }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" title="新建工单" width="640px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="类型"><el-select v-model="form.type"><el-option label="资产登记" value="asset_register" /><el-option label="资产变更" value="asset_change" /><el-option label="资产下线/报废" value="asset_retire" /><el-option label="权限/维护申请" value="maintenance" /></el-select></el-form-item>
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="优先级"><el-select v-model="form.priority"><el-option label="低" value="low" /><el-option label="普通" value="normal" /><el-option label="高" value="high" /><el-option label="紧急" value="urgent" /></el-select></el-form-item>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="4" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存草稿</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="drawerVisible" title="工单详情" size="520px">
      <template v-if="detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="标题">{{ detail.title }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ detail.status }}</el-descriptions-item>
          <el-descriptions-item label="说明">{{ detail.description }}</el-descriptions-item>
          <el-descriptions-item label="结果">{{ detail.result || '-' }}</el-descriptions-item>
        </el-descriptions>
        <el-timeline class="timeline">
          <el-timeline-item v-for="record in detail.records" :key="record.id" :timestamp="record.createdAt">
            {{ record.actor?.name }} {{ record.action }}：{{ record.remark || '-' }}
          </el-timeline-item>
        </el-timeline>
      </template>
    </el-drawer>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { api } from '../api'

const user = JSON.parse(localStorage.getItem('user') || 'null')
const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const drawerVisible = ref(false)
const detail = ref(null)
const form = reactive({ type: 'asset_register', title: '', priority: 'normal', description: '' })

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/tickets')
    items.value = data.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, { type: 'asset_register', title: '', priority: 'normal', description: '' })
  dialogVisible.value = true
}

async function save() {
  await api.post('/tickets', form)
  ElMessage.success('已创建草稿')
  dialogVisible.value = false
  load()
}

async function view(row) {
  const { data } = await api.get(`/tickets/${row.id}`)
  detail.value = data
  drawerVisible.value = true
}

function actions(row) {
  const role = user?.role
  const items = []
  if (row.status === 'draft' && (role === 'admin' || row.applicantId === user?.id)) items.push({ name: 'submit', label: '提交' })
  if (row.status === 'submitted' && ['admin', 'approver'].includes(role)) items.push({ name: 'approve', label: '通过' }, { name: 'reject', label: '驳回' })
  if (row.status === 'approved' && ['admin', 'asset_manager'].includes(role)) items.push({ name: 'start', label: '开始' })
  if (row.status === 'in_progress' && ['admin', 'asset_manager'].includes(role)) items.push({ name: 'complete', label: '完成' })
  if (row.status === 'done' && ['admin', 'asset_manager', 'applicant'].includes(role)) items.push({ name: 'close', label: '关闭' })
  if (row.status === 'draft' && (role === 'admin' || row.applicantId === user?.id)) items.push({ name: 'cancel', label: '取消' })
  return items
}

async function doAction(row, action) {
  const { value } = await ElMessageBox.prompt('处理备注', '工单操作', { inputType: 'textarea', inputValue: action })
  await api.post(`/tickets/${row.id}/${action}`, { remark: value, result: action === 'complete' ? value : '' })
  ElMessage.success('操作成功')
  load()
}

onMounted(load)
</script>

<style scoped>
.timeline {
  margin-top: 24px;
}
</style>
