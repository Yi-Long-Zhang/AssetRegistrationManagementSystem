<template>
  <section class="page">
    <PageHeader title="IP 地址池">
      <template #actions>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增网段</el-button>
      </template>
    </PageHeader>

    <div class="panel">
      <el-table :data="items" v-loading="loading" border>
        <el-table-column prop="name" label="网段名称" min-width="140" />
        <el-table-column prop="cidr" label="CIDR" width="150" />
        <el-table-column prop="description" label="用途说明" min-width="200" show-overflow-tooltip />
        <el-table-column label="创建时间" width="170"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
        <el-table-column label="操作" width="210" align="center">
          <template #default="{ row }">
            <el-button link type="primary" @click="openUsage(row)">使用情况</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <ConfirmAction link type="danger" :message="`确认删除网段 ${row.name}？`" @confirm="remove(row)">删除</ConfirmAction>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新增/编辑网段 -->
    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑网段' : '新增网段'" width="480px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="网段名称"><el-input v-model="form.name" placeholder="如：生产网段 A" /></el-form-item>
        <el-form-item label="CIDR"><el-input v-model="form.cidr" placeholder="如：10.0.0.0/24" /></el-form-item>
        <el-form-item label="用途说明"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 使用情况 -->
    <el-dialog v-model="usageVisible" :title="`网段使用情况：${usage?.segment?.name || ''}`" width="640px">
      <div v-if="usage" class="usage-stats">
        <div class="usage-stat"><span>总地址</span><strong>{{ usage.total }}</strong></div>
        <div class="usage-stat"><span>已用</span><strong class="text-warn">{{ usage.used }}</strong></div>
        <div class="usage-stat"><span>可用</span><strong class="text-ok">{{ usage.available }}</strong></div>
        <div class="usage-stat"><span>冲突</span><strong class="text-danger">{{ usage.conflicts?.length || 0 }}</strong></div>
      </div>
      <el-alert
        v-if="usage?.conflicts?.length"
        type="error"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
        :title="`发现 ${usage.conflicts.length} 处 IP 占用冲突：`"
      >
        <span v-for="conflict in usage.conflicts" :key="conflict.ip" class="conflict-item">
          {{ conflict.ip }}（{{ conflict.assets?.join(' / ') }}）
        </span>
      </el-alert>
      <el-table v-if="usage?.usedIPs?.length" :data="usage.usedIPs" size="small" border max-height="320">
        <el-table-column label="IP 地址" width="140">
          <template #default="{ row }">
            <el-tag v-if="isConflicted(row.ip)" type="danger" size="small">{{ row.ip }}</el-tag>
            <span v-else>{{ row.ip }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="assetNo" label="资产编号" min-width="140" />
        <el-table-column prop="hostname" label="主机名" min-width="140" />
      </el-table>
      <el-empty v-else-if="usage && !usage.usedIPs.length" description="该网段暂无资产占用" :image-size="60" />
      <template #footer>
        <el-button @click="usageVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { ipSegmentsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'
import ConfirmAction from '../components/common/ConfirmAction.vue'

const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const usageVisible = ref(false)
const usage = ref(null)
const form = reactive(emptyForm())

function emptyForm() {
  return { id: undefined, name: '', cidr: '', description: '' }
}

async function load() {
  loading.value = true
  try {
    const data = await ipSegmentsApi.list()
    items.value = data.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}

function openEdit(row) {
  Object.assign(form, emptyForm(), { id: row.id, name: row.name, cidr: row.cidr, description: row.description })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim() || !form.cidr.trim()) return ElMessage.warning('请填写网段名称与 CIDR')
  submitting.value = true
  try {
    if (form.id) await ipSegmentsApi.update(form.id, { name: form.name, cidr: form.cidr, description: form.description })
    else await ipSegmentsApi.create({ name: form.name, cidr: form.cidr, description: form.description })
    ElMessage.success('已保存')
    dialogVisible.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

async function remove(row) {
  try {
    await ipSegmentsApi.remove(row.id)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

async function openUsage(row) {
  try {
    usage.value = await ipSegmentsApi.usage(row.id)
    usageVisible.value = true
  } catch (e) {
    ElMessage.error(e.message || '加载使用情况失败')
  }
}

function isConflicted(ip) {
  return usage.value?.conflicts?.some((c) => c.ip === ip)
}

function formatTime(value) {
  return value ? String(value).replace('T', ' ').slice(0, 16) : '-'
}

onMounted(load)
</script>

<style scoped>
.usage-stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 14px;
}
.usage-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: #f8f9fc;
  border: 1px solid #e5e9f5;
  border-radius: 10px;
  padding: 12px;
  text-align: center;
}
.usage-stat span {
  font-size: 12px;
  color: #6b7280;
}
.usage-stat strong {
  font-size: 22px;
  color: #1f2937;
}
.text-warn {
  color: #d97706;
}
.text-ok {
  color: #059669;
}
.text-danger {
  color: #dc2626;
}
.conflict-item {
  display: block;
  margin-top: 4px;
  font-size: 13px;
}
</style>
