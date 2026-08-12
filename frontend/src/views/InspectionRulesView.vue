<template>
  <section class="page">
    <PageHeader title="巡检规则" />
    <div class="panel">
      <div class="toolbar">
        <h3>定期巡检规则（按频率自动生成巡检工单草稿）</h3>
        <el-button type="primary" @click="openCreate">新建规则</el-button>
      </div>
      <el-table :data="items" v-loading="loading" stripe>
        <el-table-column prop="name" label="规则名称" min-width="160" />
        <el-table-column label="频率" width="120">
          <template #default="{ row }">{{ frequencyLabel(row.frequency) }} <span v-if="row.frequency === 'weekly'">(周{{ weekLabel(row.dayOfWeek) }})</span><span v-else-if="row.frequency === 'monthly'">({{ row.dayOfMonth }}日)</span></template>
        </el-table-column>
        <el-table-column label="执行时间" width="100">
          <template #default="{ row }">{{ row.timeOfDay }}</template>
        </el-table-column>
        <el-table-column label="执行人" width="120">
          <template #default="{ row }">{{ row.assignee?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="上次生成" width="160">
          <template #default="{ row }">{{ row.lastRunAt || '-' }}</template>
        </el-table-column>
        <el-table-column label="启用" width="90">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="(value) => toggle(row, value)" />
          </template>
        </el-table-column>
        <el-table-column prop="description" label="巡检内容" min-width="180" show-overflow-tooltip />
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button link type="primary" @click="testRun(row)">试运行</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑巡检规则' : '新建巡检规则'" width="560px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="规则名称" required><el-input v-model="form.name" placeholder="如：机房每周巡检" /></el-form-item>
        <el-form-item label="频率" required>
          <el-select v-model="form.frequency" style="width: 100%">
            <el-option label="每天" value="daily" />
            <el-option label="每周" value="weekly" />
            <el-option label="每月" value="monthly" />
          </el-select>
        </el-form-item>
        <template v-if="form.frequency === 'weekly'">
          <el-form-item label="周执行日" required>
            <el-select v-model="form.dayOfWeek" style="width: 100%">
              <el-option v-for="(label, index) in ['周日', '周一', '周二', '周三', '周四', '周五', '周六']" :key="index" :label="label" :value="index" />
            </el-select>
          </el-form-item>
        </template>
        <template v-else-if="form.frequency === 'monthly'">
          <el-form-item label="月执行日" required>
            <el-input-number v-model="form.dayOfMonth" :min="1" :max="31" />
          </el-form-item>
        </template>
        <el-form-item label="执行时间" required>
          <el-time-select v-model="form.timeOfDay" start="00:00" step="00:30" end="23:30" placeholder="选择时间" />
        </el-form-item>
        <el-form-item label="执行人" required>
          <el-select v-model="form.assigneeId" style="width: 100%">
            <el-option v-for="user in activeUsers" :key="user.id" :label="`${user.name} (${user.username})`" :value="user.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="巡检内容"><el-input v-model="form.description" type="textarea" :rows="3" placeholder="巡检内容说明，将写入工单描述" /></el-form-item>
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
import { inspectionApi, usersApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const items = ref([])
const loading = ref(false)
const users = ref([])
const dialogVisible = ref(false)
const form = reactive(blankForm())

const activeUsers = computed(() => users.value.filter((user) => user.status === 'active'))

function blankForm() {
  return { id: null, name: '', frequency: 'daily', dayOfWeek: 1, dayOfMonth: 1, timeOfDay: '09:00', assigneeId: null, description: '' }
}

function frequencyLabel(frequency) {
  return { daily: '每天', weekly: '每周', monthly: '每月' }[frequency] || frequency
}

function weekLabel(day) {
  return ['日', '一', '二', '三', '四', '五', '六'][day] ?? day
}

async function load() {
  loading.value = true
  try {
    const data = await inspectionApi.listRules()
    items.value = data.items || []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, blankForm())
  dialogVisible.value = true
}

function openEdit(row) {
  Object.assign(form, blankForm(), {
    id: row.id,
    name: row.name,
    frequency: row.frequency,
    dayOfWeek: row.dayOfWeek ?? 1,
    dayOfMonth: row.dayOfMonth ?? 1,
    timeOfDay: row.timeOfDay || '09:00',
    assigneeId: row.assigneeId,
    description: row.description
  })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('规则名称不能为空')
  if (!form.assigneeId) return ElMessage.warning('请选择执行人')
  const payload = {
    name: form.name.trim(),
    frequency: form.frequency,
    dayOfWeek: form.dayOfWeek,
    dayOfMonth: form.dayOfMonth,
    timeOfDay: form.timeOfDay,
    assigneeId: form.assigneeId,
    description: form.description,
    enabled: true
  }
  if (form.id) {
    await inspectionApi.updateRule(form.id, payload)
  } else {
    await inspectionApi.createRule(payload)
  }
  ElMessage.success('保存成功')
  dialogVisible.value = false
  await load()
}

async function toggle(row, value) {
  await inspectionApi.updateRule(row.id, {
    name: row.name,
    description: row.description,
    frequency: row.frequency,
    dayOfWeek: row.dayOfWeek,
    dayOfMonth: row.dayOfMonth,
    timeOfDay: row.timeOfDay,
    assigneeId: row.assigneeId,
    enabled: value
  })
  ElMessage.success(value ? '已启用' : '已停用')
  await load()
}

async function testRun(row) {
  await ElMessageBox.confirm(`将立即为「${row.name}」生成一张巡检工单草稿，确认？`, '试运行')
  await inspectionApi.testRule(row.id)
  ElMessage.success('已生成巡检工单，可在工单流程查看')
}

async function remove(row) {
  await ElMessageBox.confirm(`确定删除巡检规则「${row.name}」？`, '删除确认', { type: 'warning' })
  await inspectionApi.removeRule(row.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(async () => {
  const data = await usersApi.list()
  users.value = data.items || []
  await load()
})
</script>
