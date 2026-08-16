<template>
  <PageHeader title="操作审计" description="系统关键操作与变更审计日志" />
  <div class="audit-page">
    <div class="toolbar">
      <el-select v-model="filters.entity" placeholder="实体" clearable style="width: 140px" @change="load(1)">
        <el-option v-for="item in entityOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="filters.action" placeholder="动作" clearable style="width: 140px" @change="load(1)">
        <el-option v-for="item in actionOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-button @click="load(1)">查询</el-button>
    </div>

    <el-table :data="items" v-loading="loading" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="操作人" width="130">
        <template #default="{ row }">{{ row.actor?.username || '系统' }}</template>
      </el-table-column>
      <el-table-column label="实体" width="110">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ entityLabel(row.entity) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="动作" width="100" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="actionType(row.action)">{{ actionLabel(row.action) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="entityId" label="对象ID" width="90" />
      <el-table-column prop="detail" label="详情" min-width="280" show-overflow-tooltip />
      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ row.createdAt ? row.createdAt.replace('T', ' ').slice(0, 19) : '-' }}</template>
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
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import PageHeader from '../components/common/PageHeader.vue'
import { auditApi } from '../api/audit'

const entityOptions = [
  { value: 'asset', label: '资产' },
  { value: 'ticket', label: '工单' },
  { value: 'user', label: '用户' },
  { value: 'discovery_rule', label: '发现规则' },
  { value: 'discovery_run', label: '发现任务' },
  { value: 'credential', label: '凭据' },
  { value: 'software_license', label: '软件许可' },
  { value: 'ip_segment', label: 'IP地址段' },
  { value: 'backup', label: '数据备份' }
]
const actionOptions = [
  { value: 'create', label: '创建' },
  { value: 'update', label: '更新' },
  { value: 'delete', label: '删除' },
  { value: 'alert', label: '预警' },
  { value: 'adopt', label: '纳管' },
  { value: 'apply', label: '应用' },
  { value: 'offline', label: '离线' },
  { value: 'import', label: '导入' },
  { value: 'batch_delete', label: '批量删除' },
  { value: 'batch_update', label: '批量编辑' },
  { value: 'retire', label: '退役' },
  { value: 'reveal', label: '查看明文' },
  { value: 'start', label: '开始' },
  { value: 'restore', label: '恢复' },
  { value: 'change_password', label: '修改密码' },
  { value: 'ip_conflict', label: 'IP冲突' }
]

const entityLabels = Object.fromEntries(entityOptions.map((i) => [i.value, i.label]))
const actionLabels = Object.fromEntries(actionOptions.map((i) => [i.value, i.label]))

function entityLabel(v) {
  return entityLabels[v] || v || '-'
}
function actionLabel(v) {
  return actionLabels[v] || v || '-'
}
function actionType(action) {
  const map = { create: 'success', delete: 'danger', alert: 'warning', offline: 'danger', apply: 'primary' }
  return map[action] || 'info'
}

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const filters = reactive({ entity: '', action: '' })

async function load(p = 1) {
  page.value = p
  loading.value = true
  try {
    const res = await auditApi.list({ page: page.value, pageSize, entity: filters.entity, action: filters.action })
    items.value = res.items || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(() => load(1))
</script>

<style scoped>
.audit-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}
.pager {
  display: flex;
  justify-content: flex-end;
}
</style>
