<template>
  <section class="page">
    <PageHeader title="服务器资产">
      <template #actions>
        <el-upload v-if="canManage" :http-request="importAssets" :show-file-list="false" accept=".csv,.xlsx">
          <el-button :icon="Upload">导入资产</el-button>
        </el-upload>
        <el-button v-if="canManage" :icon="Download" @click="downloadTemplate">下载模板</el-button>
        <el-button v-if="canManage" :icon="Download" @click="exportAssets">批量导出</el-button>
        <el-button v-if="canManage" type="primary" :icon="Plus" @click="openCreate">新增资产</el-button>
      </template>
    </PageHeader>
    <div class="asset-stats">
      <div class="stat-card">
        <span>资产总数</span>
        <strong>{{ stats.total }}</strong>
      </div>
      <div class="stat-card">
        <span>网段数量</span>
        <strong>{{ stats.subnetCount }}</strong>
      </div>
      <div class="stat-card">
        <span>开放端口资产</span>
        <strong>{{ stats.openPortAssetCount }}</strong>
      </div>
      <div class="stat-card wide">
        <span>主要资产类型</span>
        <strong>{{ topAssetTypeText }}</strong>
      </div>
    </div>
    <DataToolbar>
      <template #actions>
        <el-input v-model="filters.q" placeholder="搜索 IP、主机名、MAC、端口、服务、负责人、网段" clearable @keyup.enter="applyFilters" />
        <el-input v-model="filters.assetType" placeholder="资产类型" clearable @keyup.enter="applyFilters" />
        <el-input v-model="filters.subnet" placeholder="所在网段" clearable @keyup.enter="applyFilters" />
        <el-input v-model="filters.owner" placeholder="负责人" clearable @keyup.enter="applyFilters" />
        <el-input v-model="filters.openPort" placeholder="端口" clearable @keyup.enter="applyFilters" />
        <el-input v-model="filters.service" placeholder="服务/应用" clearable @keyup.enter="applyFilters" />
        <el-select v-model="filters.onlineStatus" placeholder="在线状态" clearable style="width: 130px" @change="applyFilters">
          <el-option v-for="(item, key) in onlineStatusOptions" :key="key" :label="item.label" :value="key" />
        </el-select>
        <el-button :icon="Search" type="primary" @click="applyFilters">查询</el-button>
        <el-button :icon="Refresh" @click="resetFilters">重置</el-button>
        <el-popover placement="bottom-end" width="220" trigger="click">
          <template #reference>
            <el-button :icon="Setting">列设置</el-button>
          </template>
          <el-checkbox-group v-model="visibleColumnKeys">
            <el-checkbox v-for="column in columns" :key="column.key" :label="column.key" :disabled="column.required">
              {{ column.label }}
            </el-checkbox>
          </el-checkbox-group>
        </el-popover>
      </template>
    </DataToolbar>
    <div class="panel">
      <el-table :data="items" v-loading="loading" border height="560" @sort-change="handleSortChange" @row-click="openDetail">
        <el-table-column
          v-for="column in visibleColumns"
          :key="column.key"
          :prop="column.prop"
          :label="column.label"
          :width="column.width"
          :min-width="column.minWidth"
          :fixed="column.fixed"
          :sortable="column.sortable ? 'custom' : false"
          :show-overflow-tooltip="column.tooltip"
        />
        <el-table-column label="在线状态" width="90" fixed="right">
          <template #default="{ row }">
            <el-tag :type="dictItem(onlineStatusMap, row.onlineStatus).type" size="small">{{ dictItem(onlineStatusMap, row.onlineStatus).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column v-if="canManage" label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openEdit(row)">编辑</el-button>
            <ConfirmAction link type="danger" :message="`确认删除资产 ${row.ip}？`" @click.stop @confirm="remove(row)">删除</ConfirmAction>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[20, 50, 100, 200]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handlePageSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑资产' : '新增资产'" width="960px">
      <el-form :model="form" label-width="110px">
        <el-divider content-position="left">测绘资产信息</el-divider>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="序号"><el-input v-model="form.sequenceNo" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="IP地址"><el-input v-model="form.ip" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="主机名/设备"><el-input v-model="form.hostname" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="MAC地址"><el-input v-model="form.macAddress" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="厂商"><el-input v-model="form.manufacturer" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="资产类型"><el-input v-model="form.assetType" placeholder="服务器/网络设备/数据库" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="操作系统"><el-input v-model="form.os" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="资产归属"><el-input v-model="form.owner" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="所在网段"><el-input v-model="form.subnet" /></el-form-item></el-col>
        </el-row>

        <el-divider content-position="left">端口与服务</el-divider>
        <el-row :gutter="12">
          <el-col :span="24"><el-form-item label="开放端口"><el-input v-model="form.openPorts" type="textarea" :rows="2" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="运行服务/应用"><el-input v-model="form.runningServices" type="textarea" :rows="3" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="应用版本"><el-input v-model="form.appVersion" type="textarea" :rows="2" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 资产详情弹窗（弹出式 + 动画） -->
    <AssetDetailDialog v-model="detail.visible" :asset="detail.asset" />
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Download, Plus, Refresh, Search, Setting, Upload } from '@element-plus/icons-vue'
import { assetsApi } from '../api'
import AssetDetailDialog from '../components/assets/AssetDetailDialog.vue'
import ConfirmAction from '../components/common/ConfirmAction.vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import PageHeader from '../components/common/PageHeader.vue'
import { useAuthStore } from '../stores/auth'
import { canManageAssets } from '../utils/permissions'
import {
  ONLINE_STATUS_MAP,
  dictItem,
  dictOptions
} from '../constants/dictionaries'

const auth = useAuthStore()
const canManage = computed(() => canManageAssets(auth.user))
const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive(emptyAsset())
const filters = reactive({
  q: '',
  assetType: '',
  subnet: '',
  owner: '',
  manufacturer: '',
  openPort: '',
  service: '',
  onlineStatus: ''
})
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})
const sorting = reactive({
  sortBy: '',
  sortOrder: ''
})
const stats = reactive({
  total: 0,
  subnetCount: 0,
  openPortAssetCount: 0,
  byAssetType: [],
  bySubnet: [],
  byOwner: [],
  topOpenPorts: [],
  topServices: []
})
const columns = [
  { key: 'sequenceNo', prop: 'sequenceNo', label: '序号', width: 80, fixed: 'left', sortable: true },
  { key: 'ip', prop: 'ip', label: 'IP地址', width: 150, fixed: 'left', sortable: true, required: true },
  { key: 'hostname', prop: 'hostname', label: '主机名/设备名称', minWidth: 170, sortable: true },
  { key: 'macAddress', prop: 'macAddress', label: 'MAC地址', minWidth: 150, sortable: true },
  { key: 'manufacturer', prop: 'manufacturer', label: '厂商', width: 130, sortable: true },
  { key: 'assetType', prop: 'assetType', label: '资产类型', width: 120, sortable: true },
  { key: 'os', prop: 'os', label: '操作系统', minWidth: 150, sortable: true },
  { key: 'openPorts', prop: 'openPorts', label: '开放端口', minWidth: 180, sortable: true, tooltip: true },
  { key: 'runningServices', prop: 'runningServices', label: '运行服务/应用', minWidth: 260, sortable: true, tooltip: true },
  { key: 'appVersion', prop: 'appVersion', label: '应用版本', minWidth: 180, sortable: true, tooltip: true },
  { key: 'owner', prop: 'owner', label: '资产归属/负责人', width: 150, sortable: true },
  { key: 'subnet', prop: 'subnet', label: '所在网段', width: 140, sortable: true },
  { key: 'remark', prop: 'remark', label: '备注', minWidth: 220, tooltip: true }
]
const columnStorageKey = 'asset.visibleColumns'
const visibleColumnKeys = ref(loadVisibleColumnKeys())
const visibleColumns = computed(() => columns.filter((column) => visibleColumnKeys.value.includes(column.key)))
const topAssetTypeText = computed(() => {
  const top = stats.byAssetType?.[0]
  return top ? `${top.label || '未填'} ${top.count}` : '-'
})

const onlineStatusMap = ONLINE_STATUS_MAP
const onlineStatusOptions = dictOptions(ONLINE_STATUS_MAP)

const detail = reactive({ visible: false, asset: null })

async function openDetail(row) {
  detail.asset = row
  detail.visible = true
}

function emptyAsset() {
  return {
    assetNo: '',
    sequenceNo: '',
    assetType: '',
    hostname: '',
    ip: '',
    macAddress: '',
    managementIp: '',
    serialNo: '',
    manufacturer: '',
    model: '',
    status: 'in_use',
    location: '',
    rack: '',
    rackPosition: '',
    os: '',
    osVersion: '',
    cpu: '',
    memory: '',
    disk: '',
    openPorts: '',
    runningServices: '',
    appVersion: '',
    subnet: '',
    businessSystem: '',
    environment: '',
    department: '',
    owner: '',
    maintenanceVendor: '',
    purchaseDate: null,
    warrantyExpireDate: null,
    onlineDate: null,
    remark: ''
  }
}

async function load() {
  loading.value = true
  try {
    const data = await assetsApi.list(buildQuery())
    items.value = data.items
    pagination.total = data.total || 0
    pagination.page = data.page || pagination.page
    pagination.pageSize = data.pageSize || pagination.pageSize
    await loadStats()
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  const data = await assetsApi.stats(buildQuery(false))
  Object.assign(stats, {
    total: data.total || 0,
    subnetCount: data.subnetCount || 0,
    openPortAssetCount: data.openPortAssetCount || 0,
    byAssetType: data.byAssetType || [],
    bySubnet: data.bySubnet || [],
    byOwner: data.byOwner || [],
    topOpenPorts: data.topOpenPorts || [],
    topServices: data.topServices || []
  })
}

function buildQuery(includePaging = true) {
  const params = {}
  Object.entries(filters).forEach(([key, value]) => {
    if (String(value || '').trim()) params[key] = String(value).trim()
  })
  if (sorting.sortBy && sorting.sortOrder) {
    params.sortBy = sorting.sortBy
    params.sortOrder = sorting.sortOrder
  }
  if (includePaging) {
    params.page = pagination.page
    params.pageSize = pagination.pageSize
  }
  return params
}

function applyFilters() {
  pagination.page = 1
  load()
}

function resetFilters() {
  Object.keys(filters).forEach((key) => {
    filters[key] = ''
  })
  sorting.sortBy = ''
  sorting.sortOrder = ''
  pagination.page = 1
  load()
}

function handleSortChange({ prop, order }) {
  sorting.sortBy = prop || ''
  sorting.sortOrder = order === 'ascending' ? 'asc' : order === 'descending' ? 'desc' : ''
  pagination.page = 1
  load()
}

function handlePageChange(page) {
  pagination.page = page
  load()
}

function handlePageSizeChange(pageSize) {
  pagination.pageSize = pageSize
  pagination.page = 1
  load()
}

function openCreate() {
  Object.assign(form, emptyAsset(), { id: undefined })
  dialogVisible.value = true
}

function openEdit(row) {
  Object.assign(form, emptyAsset(), row)
  dialogVisible.value = true
}

async function save() {
  try {
    const payload = normalizePayload(form)
    if (form.id) await assetsApi.update(form.id, payload)
    else await assetsApi.create(payload)
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

async function downloadTemplate() {
  const data = await assetsApi.template({ format: 'xlsx' })
  saveBlob(data, 'asset-import-template.xlsx')
}

async function exportAssets() {
  const data = await assetsApi.export(buildQuery(false))
  saveBlob(data, 'assets-export.csv')
}

async function importAssets(options) {
  const formData = new FormData()
  formData.append('file', options.file)
  try {
    const data = await assetsApi.import(formData)
    ElMessage.success(`导入完成：新增 ${data.created} 条，更新 ${data.updated} 条`)
    await load()
    options.onSuccess?.(data)
  } catch (error) {
    options.onError?.(error)
    ElMessage.error(error.response?.data?.error || '导入失败')
  }
}

function saveBlob(data, filename) {
  const blob = data instanceof Blob ? data : new Blob([data])
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

function normalizePayload(value) {
  return { ...value }
}

function loadVisibleColumnKeys() {
  const defaults = columns.map((column) => column.key)
  try {
    const saved = JSON.parse(localStorage.getItem(columnStorageKey) || '[]')
    if (!Array.isArray(saved) || saved.length === 0) return defaults
    return [...new Set([...columns.filter((column) => column.required).map((column) => column.key), ...saved])].filter((key) =>
      columns.some((column) => column.key === key)
    )
  } catch {
    return defaults
  }
}

watch(
  visibleColumnKeys,
  (value) => {
    const required = columns.filter((column) => column.required).map((column) => column.key)
    const normalized = [...new Set([...required, ...value])]
    if (normalized.length !== value.length) {
      visibleColumnKeys.value = normalized
      return
    }
    localStorage.setItem(columnStorageKey, JSON.stringify(normalized))
  },
  { deep: true }
)

onMounted(load)
</script>

<style scoped>
.asset-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(160px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.stat-card {
  min-height: 72px;
  padding: 12px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.stat-card span {
  color: #64748b;
  font-size: 13px;
}

.stat-card strong {
  margin-top: 6px;
  color: #111827;
  font-size: 24px;
  font-weight: 700;
}

.stat-card.wide strong {
  font-size: 16px;
}

.panel {
  overflow: hidden;
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  padding-top: 12px;
}

:deep(.toolbar-actions) {
  grid-template-columns: repeat(auto-fit, minmax(150px, max-content));
}

@media (max-width: 900px) {
  .asset-stats {
    grid-template-columns: repeat(2, minmax(140px, 1fr));
  }
}
</style>
