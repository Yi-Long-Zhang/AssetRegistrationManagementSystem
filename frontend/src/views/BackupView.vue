<template>
  <section class="page">
    <PageHeader title="数据备份" description="数据库、附件、工单归档与配置的加密完整备份；每日自动创建并执行恢复校验" />
    <div class="panel">
      <div class="toolbar">
        <h3>备份列表</h3>
        <el-button type="primary" :loading="creating" @click="createBackup">立即备份</el-button>
      </div>
      <el-table :data="items" v-loading="loading" stripe>
        <el-table-column prop="name" label="备份文件" min-width="240" />
        <el-table-column label="大小" width="110">
          <template #default="{ row }">{{ formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="修改时间" width="190">
          <template #default="{ row }">{{ formatTime(row.modTime) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="150">
          <template #default="{ row }">
            <el-tag v-if="row.verifyError" type="danger">校验失败</el-tag>
            <el-tag v-else-if="row.verifiedAt" type="success">已校验</el-tag>
            <el-tag v-else type="info">待校验</el-tag>
            <el-tag v-if="row.offsiteCopied" type="primary" class="offsite-tag">异地副本</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="downloadBackup(row)">下载</el-button>
            <el-button link type="success" @click="verifyBackup(row)">校验</el-button>
            <el-button link type="warning" @click="restoreBackup(row)">恢复</el-button>
            <el-button link type="danger" @click="deleteBackup(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无备份，点击右上角「立即备份」创建" />
        </template>
      </el-table>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { backupsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const items = ref([])
const loading = ref(false)
const creating = ref(false)

async function load() {
  loading.value = true
  try {
    const data = await backupsApi.list()
    items.value = data.items || []
  } finally {
    loading.value = false
  }
}

async function createBackup() {
  creating.value = true
  try {
    await backupsApi.create()
    ElMessage.success('备份已创建')
    await load()
  } finally {
    creating.value = false
  }
}

async function downloadBackup(row) {
  const data = await backupsApi.download(row.name)
  const blob = data instanceof Blob ? data : new Blob([data])
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = row.name
  link.click()
  URL.revokeObjectURL(url)
}

async function restoreBackup(row) {
  await ElMessageBox.confirm(
    `确认恢复备份「${row.name}」？恢复将覆盖当前数据库，需重启后端后生效。`,
    '恢复确认',
    { type: 'warning', confirmButtonText: '标记恢复', cancelButtonText: '取消' }
  )
  const res = await backupsApi.restore(row.name)
  ElMessage.success(res.message || '已标记恢复，重启后端后生效')
}

async function verifyBackup(row) {
  await backupsApi.verify(row.name)
  ElMessage.success('备份完整性校验通过')
  await load()
}

async function deleteBackup(row) {
  await ElMessageBox.confirm(`确认删除备份「${row.name}」？删除后不可恢复。`, '删除确认', { type: 'warning' })
  await backupsApi.remove(row.name)
  ElMessage.success('备份已删除')
  await load()
}

function formatSize(bytes) {
  if (bytes === null || bytes === undefined) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(load)
</script>

<style scoped>
.offsite-tag {
  margin-left: 6px;
}
</style>
