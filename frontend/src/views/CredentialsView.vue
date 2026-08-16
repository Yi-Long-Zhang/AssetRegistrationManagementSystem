<template>
  <section class="page">
    <PageHeader title="凭据管理" description="服务器账号/密码加密托管，查看明文需二次确认并记录审计" />
    <div class="panel">
      <div class="toolbar">
        <h3>凭据列表</h3>
        <el-button type="primary" @click="openCreate">新增凭据</el-button>
      </div>
      <el-table :data="items" v-loading="loading" stripe>
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">{{ typeLabel(row.type) }}</template>
        </el-table-column>
        <el-table-column label="关联资产" min-width="160">
          <template #default="{ row }">{{ row.asset ? `${row.asset.hostname} / ${row.asset.ip}` : '-' }}</template>
        </el-table-column>
        <el-table-column label="最后查看" width="170">
          <template #default="{ row }">{{ row.lastAccessedAt ? new Date(row.lastAccessedAt).toLocaleString() : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="reveal(row)">查看</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑凭据' : '新增凭据'" width="520px">
      <el-form :model="form" label-width="120px">
        <el-form-item label="关联资产">
          <el-select v-model="form.assetId" clearable filterable placeholder="可选" style="width: 100%">
            <el-option v-for="a in assets" :key="a.id" :label="`${a.hostname} / ${a.ip}`" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称" required><el-input v-model="form.name" placeholder="如 root 登录" /></el-form-item>
        <el-form-item label="用户名" required><el-input v-model="form.username" placeholder="如 root" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width: 100%">
            <el-option v-for="(label, value) in typeMap" :key="value" :label="label" :value="value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="editing ? '密码（留空不修改）' : '密码/密钥'" :required="!editing">
          <el-input v-model="form.secret" type="password" show-password placeholder="明文，保存后 AES-GCM 加密存储" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="revealVisible" title="凭据明文" width="480px">
      <div class="reveal-box">
        <div class="reveal-row"><span class="muted">名称</span>{{ revealData.name }}</div>
        <div class="reveal-row"><span class="muted">用户名</span>{{ revealData.username }}</div>
        <div class="reveal-row"><span class="muted">密码/密钥</span><code>{{ revealSecret }}</code></div>
      </div>
      <template #footer>
        <el-button @click="copySecret">复制</el-button>
        <el-button type="primary" @click="revealVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { credentialsApi, assetsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const typeMap = { ssh: 'SSH', rdp: '远程桌面', database: '数据库', app: '应用', other: '其他' }

const items = ref([])
const assets = ref([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const revealVisible = ref(false)
const editing = ref(false)
const editingId = ref(null)
const revealData = ref({})
const revealSecret = ref('')
const form = reactive({ assetId: null, name: '', username: '', type: 'ssh', secret: '', remark: '' })

function typeLabel(t) { return typeMap[t] || t }

async function load() {
  loading.value = true
  try {
    const data = await credentialsApi.list()
    items.value = data.items || []
  } finally {
    loading.value = false
  }
}

async function loadAssets() {
  try {
    const data = await assetsApi.list({ pageSize: 500 })
    assets.value = data.items || []
  } catch {
    assets.value = []
  }
}

function openCreate() {
  editing.value = false
  editingId.value = null
  Object.assign(form, { assetId: null, name: '', username: '', type: 'ssh', secret: '', remark: '' })
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = true
  editingId.value = row.id
  Object.assign(form, { assetId: row.assetId ?? null, name: row.name, username: row.username, type: row.type, secret: '', remark: row.remark })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim() || !form.username.trim()) return ElMessage.warning('请填写名称和用户名')
  if (!editing.value && !form.secret) return ElMessage.warning('请填写密码/密钥')
  saving.value = true
  try {
    const payload = { assetId: form.assetId, name: form.name.trim(), username: form.username.trim(), type: form.type, secret: form.secret, remark: form.remark }
    if (editing.value) await credentialsApi.update(editingId.value, payload)
    else await credentialsApi.create(payload)
    ElMessage.success(editing.value ? '凭据已更新' : '凭据已创建')
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function reveal(row) {
  await ElMessageBox.confirm(`确认查看「${row.name}」的明文？此操作会记录审计。`, '查看确认', { type: 'warning' })
  const data = await credentialsApi.reveal(row.id)
  revealData.value = data.credential || row
  revealSecret.value = data.secret
  revealVisible.value = true
  await load()
}

async function copySecret() {
  try {
    await navigator.clipboard.writeText(revealSecret.value)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择复制')
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除凭据「${row.name}」？删除后不可恢复。`, '删除确认', { type: 'warning' })
  await credentialsApi.remove(row.id)
  ElMessage.success('凭据已删除')
  await load()
}

onMounted(() => { load(); loadAssets() })
</script>

<style scoped>
.reveal-box { display: flex; flex-direction: column; gap: 12px; }
.reveal-row { display: flex; gap: 12px; align-items: baseline; }
.reveal-row code { background: #f5f7fa; padding: 4px 10px; border-radius: 6px; word-break: break-all; }
.muted { color: #909399; min-width: 70px; }
</style>
