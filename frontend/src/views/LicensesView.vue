<template>
  <section class="page">
    <PageHeader title="软件许可" description="软件许可证台账，密钥 AES-GCM 加密托管，到期前自动提醒" />
    <div class="panel">
      <div class="toolbar">
        <h3>许可证列表</h3>
        <div class="toolbar-actions">
          <el-upload :http-request="importLicenses" :show-file-list="false" accept=".csv,.xlsx">
            <el-button :icon="Upload">导入</el-button>
          </el-upload>
          <el-button :icon="Download" @click="downloadTemplate">下载模板</el-button>
          <el-button :icon="Download" @click="exportLicenses">批量导出</el-button>
          <el-button type="primary" :icon="Plus" @click="openCreate">新增许可证</el-button>
        </div>
      </div>
      <el-table :data="items" v-loading="loading" stripe>
        <el-table-column prop="name" label="软件名" min-width="160" />
        <el-table-column prop="vendor" label="厂商" width="140">
          <template #default="{ row }">{{ row.vendor || '-' }}</template>
        </el-table-column>
        <el-table-column label="类型" width="110">
          <template #default="{ row }">
            <el-tag :type="typeTag(row.type)" size="small" effect="plain">{{ typeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="授权数量" width="130">
          <template #default="{ row }">
            <span :class="{ 'seat-full': row.totalSeats > 0 && row.usedSeats >= row.totalSeats }">{{ row.usedSeats }} / {{ row.totalSeats }}</span>
          </template>
        </el-table-column>
        <el-table-column label="到期日" width="160">
          <template #default="{ row }">
            <el-tag v-if="expiryStatus(row)" :type="expiryStatus(row).type" size="small" effect="plain">{{ expiryStatus(row).text }} {{ fmtDate(row.expireDate) }}</el-tag>
            <span v-else>{{ fmtDate(row.expireDate) || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="关联资产" min-width="150">
          <template #default="{ row }">{{ row.asset ? `${row.asset.hostname} / ${row.asset.ip}` : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openAttachments(row)">附件</el-button>
            <el-button link type="primary" @click="reveal(row)">查看密钥</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑许可证' : '新增许可证'" width="560px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="软件名" required><el-input v-model="form.name" placeholder="如 Windows Server 2022" /></el-form-item>
        <el-form-item label="厂商"><el-input v-model="form.vendor" placeholder="如 Microsoft" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.type" style="width: 100%">
            <el-option v-for="(label, value) in typeMap" :key="value" :label="label" :value="value" />
          </el-select>
        </el-form-item>
        <el-form-item label="关联资产">
          <el-select v-model="form.assetId" clearable filterable placeholder="可选" style="width: 100%">
            <el-option v-for="a in assets" :key="a.id" :label="`${a.hostname} / ${a.ip}`" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="授权数量">
          <div class="seat-row">
            <el-input-number v-model="form.totalSeats" :min="0" controls-position="right" style="width: 120px" />
            <span class="seat-sep">已用</span>
            <el-input-number v-model="form.usedSeats" :min="0" :max="form.totalSeats || undefined" controls-position="right" style="width: 120px" />
          </div>
        </el-form-item>
        <el-form-item label="到期日">
          <el-date-picker v-model="form.expireDate" type="date" value-format="YYYY-MM-DD" placeholder="可选" style="width: 100%" />
        </el-form-item>
        <el-form-item label="购买日期">
          <el-date-picker v-model="form.purchaseDate" type="date" value-format="YYYY-MM-DD" placeholder="可选" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="editing ? '许可证密钥（留空不修改）' : '许可证密钥'" :required="!editing">
          <el-input v-model="form.licenseKey" type="textarea" :rows="2" placeholder="明文，保存后 AES-GCM 加密存储" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="revealVisible" title="许可证密钥明文" width="520px">
      <div class="reveal-box">
        <div class="reveal-row"><span class="muted">软件名</span>{{ revealData.name }}</div>
        <div class="reveal-row"><span class="muted">类型</span>{{ typeLabel(revealData.type) }}</div>
        <div class="reveal-row"><span class="muted">到期日</span>{{ fmtDate(revealData.expireDate) || '-' }}</div>
        <div class="reveal-row"><span class="muted">许可证密钥</span><code>{{ revealKey }}</code></div>
      </div>
      <template #footer>
        <el-button @click="copyKey">复制</el-button>
        <el-button type="primary" @click="revealVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="attachmentsVisible" :title="`附件：${currentLicense?.name || ''}`" size="460px">
      <el-upload :http-request="uploadAttachment" :show-file-list="false">
        <el-button>上传附件</el-button>
      </el-upload>
      <el-table class="attachment-table" :data="attachments" size="small" v-loading="attachmentsLoading">
        <el-table-column prop="originalName" label="文件名" min-width="180" />
        <el-table-column prop="size" label="大小" width="90">
          <template #default="{ row }">{{ formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="上传人" width="90"><template #default="{ row }">{{ row.uploader?.name }}</template></el-table-column>
        <el-table-column label="操作" width="110">
          <template #default="{ row }">
            <el-button link type="primary" @click="downloadAttachment(row)">下载</el-button>
            <el-button link type="danger" @click="removeAttachment(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Download, Plus, Upload } from '@element-plus/icons-vue'
import { licensesApi, assetsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const typeMap = { commercial: '商业授权', 'open-source': '开源', subscription: '订阅制', other: '其他' }
const typeTags = { commercial: 'primary', 'open-source': 'success', subscription: 'warning', other: 'info' }

const items = ref([])
const assets = ref([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const revealVisible = ref(false)
const editing = ref(false)
const editingId = ref(null)
const revealData = ref({})
const revealKey = ref('')
const attachmentsVisible = ref(false)
const attachmentsLoading = ref(false)
const attachments = ref([])
const currentLicense = ref(null)
const form = reactive({ assetId: null, name: '', vendor: '', type: 'commercial', licenseKey: '', totalSeats: 0, usedSeats: 0, expireDate: '', purchaseDate: '', remark: '' })

function typeLabel(t) { return typeMap[t] || t }
function typeTag(t) { return typeTags[t] || 'info' }

// fmtDate 兼容后端时间字符串与 YYYY-MM-DD。
function fmtDate(v) {
  if (!v) return ''
  return String(v).slice(0, 10)
}

// expiryStatus 计算许可证到期状态：已过期/30 天内到期高亮。
function expiryStatus(row) {
  if (!row.expireDate) return null
  const expire = new Date(row.expireDate).getTime()
  const diff = expire - Date.now()
  if (diff < 0) return { text: '已过期', type: 'danger' }
  if (diff <= 30 * 86400000) return { text: '即将到期', type: 'warning' }
  return null
}

function formatSize(bytes) {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

async function load() {
  loading.value = true
  try {
    const data = await licensesApi.list()
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
  Object.assign(form, { assetId: null, name: '', vendor: '', type: 'commercial', licenseKey: '', totalSeats: 0, usedSeats: 0, expireDate: '', purchaseDate: '', remark: '' })
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = true
  editingId.value = row.id
  Object.assign(form, {
    assetId: row.assetId ?? null,
    name: row.name,
    vendor: row.vendor,
    type: row.type,
    licenseKey: '',
    totalSeats: row.totalSeats || 0,
    usedSeats: row.usedSeats || 0,
    expireDate: fmtDate(row.expireDate),
    purchaseDate: fmtDate(row.purchaseDate),
    remark: row.remark
  })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) return ElMessage.warning('请填写软件名')
  if (!editing.value && !form.licenseKey.trim()) return ElMessage.warning('请填写许可证密钥')
  saving.value = true
  try {
    const payload = {
      assetId: form.assetId,
      name: form.name.trim(),
      vendor: form.vendor.trim(),
      type: form.type,
      licenseKey: form.licenseKey.trim(),
      totalSeats: form.totalSeats,
      usedSeats: form.usedSeats,
      expireDate: form.expireDate,
      purchaseDate: form.purchaseDate,
      remark: form.remark
    }
    if (editing.value) await licensesApi.update(editingId.value, payload)
    else await licensesApi.create(payload)
    ElMessage.success(editing.value ? '许可证已更新' : '许可证已创建')
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function reveal(row) {
  await ElMessageBox.confirm(`确认查看「${row.name}」的许可证密钥？此操作会记录审计。`, '查看确认', { type: 'warning' })
  const data = await licensesApi.reveal(row.id)
  revealData.value = data.license || row
  revealKey.value = data.licenseKey
  revealVisible.value = true
  await load()
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(revealKey.value)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择复制')
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除许可证「${row.name}」？删除后不可恢复。`, '删除确认', { type: 'warning' })
  await licensesApi.remove(row.id)
  ElMessage.success('许可证已删除')
  await load()
}

// ---------- 导入/导出 ----------
async function importLicenses(options) {
  const formData = new FormData()
  formData.append('file', options.file)
  try {
    const data = await licensesApi.import(formData)
    ElMessage.success(`导入完成：新增 ${data.created} 条，更新 ${data.updated} 条`)
    options.onSuccess?.(data)
    await load()
  } catch (error) {
    options.onError?.(error)
    ElMessage.error(error.response?.data?.error || '导入失败')
  }
}

async function downloadTemplate() {
  const data = await licensesApi.template({ format: 'xlsx' })
  saveBlob(data, 'license-import-template.xlsx')
}

async function exportLicenses() {
  const data = await licensesApi.export()
  saveBlob(data, 'licenses-export.csv')
}

function saveBlob(data, filename) {
  const url = URL.createObjectURL(data)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

// ---------- 附件 ----------
async function openAttachments(row) {
  currentLicense.value = row
  attachmentsVisible.value = true
  await loadAttachments()
}

async function loadAttachments() {
  if (!currentLicense.value) return
  attachmentsLoading.value = true
  try {
    const data = await licensesApi.attachments(currentLicense.value.id)
    attachments.value = data.items || []
  } finally {
    attachmentsLoading.value = false
  }
}

async function uploadAttachment(options) {
  if (!currentLicense.value) return
  const formData = new FormData()
  formData.append('file', options.file)
  try {
    await licensesApi.uploadAttachment(currentLicense.value.id, formData)
    ElMessage.success('附件已上传')
    options.onSuccess?.()
    await loadAttachments()
  } catch (error) {
    options.onError?.(error)
    ElMessage.error(error.response?.data?.error || '上传失败')
  }
}

async function downloadAttachment(row) {
  if (!currentLicense.value) return
  const data = await licensesApi.downloadAttachment(currentLicense.value.id, row.id)
  saveBlob(data, row.originalName)
}

async function removeAttachment(row) {
  if (!currentLicense.value) return
  await ElMessageBox.confirm(`确认删除附件「${row.originalName}」？`, '删除确认', { type: 'warning' })
  await licensesApi.removeAttachment(currentLicense.value.id, row.id)
  ElMessage.success('附件已删除')
  await loadAttachments()
}

onMounted(() => { load(); loadAssets() })
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.reveal-box { display: flex; flex-direction: column; gap: 12px; }
.reveal-row { display: flex; gap: 12px; align-items: baseline; }
.reveal-row code { background: #f5f7fa; padding: 4px 10px; border-radius: 6px; word-break: break-all; }
.muted { color: #909399; min-width: 80px; }
.seat-row { display: flex; align-items: center; gap: 10px; }
.seat-sep { color: #909399; }
.seat-full { color: #f56c6c; font-weight: 600; }
.attachment-table { margin-top: 12px; }
</style>
