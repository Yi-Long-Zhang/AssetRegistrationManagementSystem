<template>
  <section class="page">
    <PageHeader title="资产盘点" />
    <div class="panel">
      <div class="toolbar">
        <h3>盘点单</h3>
        <el-button type="primary" @click="openCreate">新建盘点单</el-button>
      </div>
      <el-table :data="items" v-loading="loading" stripe>
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'closed' ? 'info' : 'success'" size="small">{{ row.status === 'closed' ? '已关闭' : '盘点中' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="total" label="资产数" width="90" />
        <el-table-column prop="checked" label="已核对" width="90" />
        <el-table-column label="盘亏" width="90">
          <template #default="{ row }"><span :class="{ 'missing-num': row.missing > 0 }">{{ row.missing }}</span></template>
        </el-table-column>
        <el-table-column label="创建人" width="120"><template #default="{ row }">{{ row.creator?.name || '-' }}</template></el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="170" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">盘点</el-button>
            <el-button link type="primary" @click="exportCsv(row)" :disabled="row.status === 'in_progress'">导出</el-button>
            <el-button link type="danger" @click="closeTask(row)" :disabled="row.status === 'closed'">关闭</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" title="新建盘点单" width="480px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称" required><el-input v-model="form.name" placeholder="如：2026 年度资产盘点" /></el-form-item>
        <el-form-item label="资产类型">
          <el-select v-model="form.assetType" clearable placeholder="全部" style="width: 100%">
            <el-option label="服务器" value="server" />
            <el-option label="数据库" value="database" />
            <el-option label="网络设备" value="network" />
            <el-option label="工作站" value="workstation" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="create">创建</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="drawerVisible" title="盘点明细" size="640px">
      <template v-if="detail">
        <div class="detail-head">
          <strong>{{ detail.name }}</strong>
          <span class="muted">{{ detail.status === 'closed' ? '已关闭' : '盘点中' }}</span>
        </div>
        <el-table :data="detail.items" size="small" max-height="560">
          <el-table-column label="资产编号" min-width="120"><template #default="{ row }">{{ row.asset?.assetNo || '-' }}</template></el-table-column>
          <el-table-column label="主机名/IP" min-width="160"><template #default="{ row }">{{ row.asset?.hostname }} / {{ row.asset?.ip }}</template></el-table-column>
          <el-table-column label="结果" width="90">
            <template #default="{ row }">
              <el-tag :type="resultTag(row.result)" size="small">{{ resultLabel(row.result) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="备注" min-width="120" prop="remark" show-overflow-tooltip />
          <el-table-column label="操作" width="150" fixed="right">
            <template #default="{ row }">
              <template v-if="detail.status !== 'closed'">
                <el-button link type="success" @click="checkItem(row, 'matched')">一致</el-button>
                <el-button link type="danger" @click="checkItem(row, 'missing')">盘亏</el-button>
              </template>
              <span v-else>-</span>
            </template>
          </el-table-column>
        </el-table>
      </template>
    </el-drawer>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { stocktakeApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const drawerVisible = ref(false)
const detail = ref(null)
const form = reactive({ name: '', assetType: '', remark: '' })

function resultLabel(result) {
  return { pending: '未核对', matched: '一致', missing: '盘亏' }[result] || result
}
function resultTag(result) {
  return { pending: 'info', matched: 'success', missing: 'danger' }[result] || 'info'
}

async function load() {
  loading.value = true
  try {
    const data = await stocktakeApi.list()
    items.value = data.items || []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, { name: '', assetType: '', remark: '' })
  dialogVisible.value = true
}

async function create() {
  if (!form.name.trim()) return ElMessage.warning('请输入名称')
  await stocktakeApi.create({ name: form.name.trim(), assetType: form.assetType || '', remark: form.remark })
  ElMessage.success('盘点单已创建')
  dialogVisible.value = false
  await load()
}

async function openDetail(row) {
  detail.value = await stocktakeApi.detail(row.id)
  drawerVisible.value = true
}

async function checkItem(row, result) {
  const remark = result === 'missing' ? (await ElMessageBox.prompt('盘亏原因（可选）', '盘亏', { inputValue: '' }).then((r) => r.value).catch(() => null)) : ''
  if (result === 'missing' && remark === null) return
  await stocktakeApi.checkItem(detail.value.id, row.id, { result, remark })
  ElMessage.success('已核对')
  detail.value = await stocktakeApi.detail(detail.value.id)
}

async function closeTask(row) {
  await ElMessageBox.confirm(`确认关闭盘点单「${row.name}」？关闭后不可再修改。`, '关闭确认', { type: 'warning' })
  const summary = await stocktakeApi.close(row.id)
  ElMessage.success(`已关闭：一致 ${summary.matched}，盘亏 ${summary.missing}`)
  await load()
}

async function exportCsv(row) {
  const res = await stocktakeApi.exportCsv(row.id)
  const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `stocktake-${row.id}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped>
.missing-num {
  color: #f56c6c;
  font-weight: 600;
}

.detail-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}

.muted {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
</style>
