<template>
  <section class="page">
    <PageHeader title="任务运行中心" description="查看调度任务执行状态、失败原因与重试记录" />

    <div class="toolbar">
      <el-select v-model="filters.kind" clearable placeholder="任务类型" style="width: 180px" @change="load(1)">
        <el-option v-for="item in kindOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="filters.status" clearable placeholder="状态" style="width: 140px" @change="load(1)">
        <el-option label="排队中" value="queued" />
        <el-option label="运行中" value="running" />
        <el-option label="成功" value="succeeded" />
        <el-option label="失败" value="failed" />
      </el-select>
      <el-button :icon="Refresh" @click="load(page)">刷新</el-button>
    </div>

    <el-table :data="items" v-loading="loading" stripe border @row-click="showDetail">
      <el-table-column prop="id" label="ID" width="72" />
      <el-table-column label="任务" min-width="150">
        <template #default="{ row }">{{ kindLabel(row.kind) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="source" label="来源" width="100" />
      <el-table-column label="尝试" width="90">
        <template #default="{ row }">{{ row.attempts }} / {{ row.maxAttempts }}</template>
      </el-table-column>
      <el-table-column prop="error" label="结果 / 错误" min-width="260" show-overflow-tooltip>
        <template #default="{ row }">{{ row.error || row.result || '-' }}</template>
      </el-table-column>
      <el-table-column label="开始时间" width="180">
        <template #default="{ row }">{{ formatTime(row.startedAt || row.scheduledAt) }}</template>
      </el-table-column>
      <el-table-column label="耗时" width="100">
        <template #default="{ row }">{{ duration(row) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.status === 'failed'"
            link
            type="primary"
            :icon="RefreshRight"
            @click.stop="retry(row)"
          >
            重试
          </el-button>
          <el-button
            v-if="row.status === 'failed' && !row.acknowledgedAt"
            link
            type="success"
            :icon="CircleCheck"
            @click.stop="acknowledge(row)"
          >
            确认
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        layout="total, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="load"
      />
    </div>

    <el-dialog v-model="detailVisible" title="任务详情" width="680px">
      <el-descriptions v-if="selected" :column="2" border>
        <el-descriptions-item label="任务 ID">{{ selected.id }}</el-descriptions-item>
        <el-descriptions-item label="类型">{{ kindLabel(selected.kind) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ statusLabel(selected.status) }}</el-descriptions-item>
        <el-descriptions-item label="来源">{{ selected.source }}</el-descriptions-item>
        <el-descriptions-item label="排队时间">{{ formatTime(selected.scheduledAt) }}</el-descriptions-item>
        <el-descriptions-item label="完成时间">{{ formatTime(selected.finishedAt) }}</el-descriptions-item>
        <el-descriptions-item label="唯一键" :span="2">{{ selected.uniqueKey }}</el-descriptions-item>
        <el-descriptions-item label="结果" :span="2">
          <pre>{{ selected.result || '-' }}</pre>
        </el-descriptions-item>
        <el-descriptions-item label="错误" :span="2">
          <pre class="error-text">{{ selected.error || '-' }}</pre>
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { CircleCheck, Refresh, RefreshRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { tasksApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const kindOptions = [
  { value: 'discovery_scan', label: '资产发现' },
  { value: 'sla_overdue', label: 'SLA 超时' },
  { value: 'inspection_ticket', label: '巡检建单' },
  { value: 'warranty_reminder', label: '维保提醒' },
  { value: 'license_reminder', label: '许可提醒' },
  { value: 'complete_backup', label: '完整备份' }
]
const kindLabels = Object.fromEntries(kindOptions.map((item) => [item.value, item.label]))
const statusLabels = { queued: '排队中', running: '运行中', succeeded: '成功', failed: '失败' }
const statusTypes = { queued: 'info', running: 'warning', succeeded: 'success', failed: 'danger' }

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const filters = reactive({ kind: '', status: '' })
const detailVisible = ref(false)
const selected = ref(null)

async function load(targetPage = 1) {
  page.value = targetPage
  loading.value = true
  try {
    const data = await tasksApi.list({ ...filters, page: page.value, pageSize })
    items.value = data.items || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function retry(row) {
  await tasksApi.retry(row.id)
  ElMessage.success('任务已重试')
  await load(page.value)
}

async function acknowledge(row) {
  await tasksApi.acknowledge(row.id)
  ElMessage.success('失败任务已确认')
  await load(page.value)
}

async function showDetail(row) {
  selected.value = await tasksApi.get(row.id)
  detailVisible.value = true
}

function kindLabel(value) {
  return kindLabels[value] || value
}

function statusLabel(value) {
  return statusLabels[value] || value
}

function statusType(value) {
  return statusTypes[value] || 'info'
}

function formatTime(value) {
  return value ? new Date(value).toLocaleString() : '-'
}

function duration(row) {
  if (!row.startedAt) return '-'
  const end = row.finishedAt ? new Date(row.finishedAt) : new Date()
  const seconds = Math.max(0, Math.round((end - new Date(row.startedAt)) / 1000))
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}

onMounted(() => load(1))
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font: inherit;
}

.error-text {
  color: var(--el-color-danger);
}
</style>
