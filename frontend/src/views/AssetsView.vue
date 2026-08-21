<template>
  <section class="page">
    <PageHeader title="服务器资产">
      <template #actions>
        <el-upload v-if="canManage" :http-request="importAssets" :show-file-list="false" accept=".csv,.xlsx">
          <el-button :icon="Upload">导入资产</el-button>
        </el-upload>
        <el-button v-if="canManage" :icon="Download" @click="downloadTemplate">下载模板</el-button>
        <el-button v-if="canManage" :icon="Download" @click="exportAssets">批量导出</el-button>
        <el-button v-if="canManage" :icon="Download" @click="exportStatsReport">报表导出</el-button>
        <el-button v-if="canManage" :icon="Delete" type="danger" plain :disabled="!selectedIds.length" @click="batchDelete">批量删除{{ selectedIds.length ? ` (${selectedIds.length})` : '' }}</el-button>
        <el-button v-if="canManage" :icon="EditPen" plain :disabled="!selectedIds.length" @click="openBatchEdit">批量编辑{{ selectedIds.length ? ` (${selectedIds.length})` : '' }}</el-button>
        <el-button v-if="canManage" :icon="Printer" plain :disabled="!selectedIds.length" @click="printLabels">打印标签</el-button>
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
        <div class="toolbar-scroll">
          <template v-for="field in visibleFilterFields" :key="field.key">
            <el-input
              v-if="field.component === 'input'"
              v-model="filters[field.key]"
              :placeholder="field.placeholder"
              clearable
              :style="{ width: field.width + 'px' }"
              @keyup.enter="applyFilters"
            />
            <el-select
              v-else
              v-model="filters[field.key]"
              :placeholder="field.placeholder"
              clearable
              :style="{ width: field.width + 'px' }"
              @change="applyFilters"
            >
              <el-option v-for="item in onlineStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </template>
          <el-button :icon="Search" type="primary" @click="applyFilters">查询</el-button>
          <el-button :icon="Refresh" @click="resetFilters">重置</el-button>
          <el-popover placement="bottom-end" width="240" trigger="click">
            <template #reference>
              <el-button :icon="Setting">筛选</el-button>
            </template>
            <div class="filter-settings-title">显示筛选字段（自动保存）</div>
            <el-checkbox-group v-model="filterFieldKeys">
              <el-checkbox v-for="field in filterFields" :key="field.key" :value="field.key">
                {{ field.label }}
              </el-checkbox>
            </el-checkbox-group>
          </el-popover>
          <el-popover placement="bottom-end" width="220" trigger="click">
            <template #reference>
              <el-button :icon="Setting">列设置</el-button>
            </template>
            <el-checkbox-group v-model="visibleColumnKeys">
              <el-checkbox v-for="column in columns" :key="column.key" :value="column.key" :disabled="column.required">
                {{ column.label }}
              </el-checkbox>
            </el-checkbox-group>
          </el-popover>
        </div>
      </template>
    </DataToolbar>
    <div v-if="activeFilterChips.length" class="filter-chips">
      <span class="filter-chips-label">已筛选：</span>
      <el-tag v-for="chip in activeFilterChips" :key="chip.key" size="small" closable @close="clearFilter(chip.key)">
        {{ chip.label }}：{{ chip.value }}
      </el-tag>
      <el-button link type="primary" size="small" @click="resetFilters">清除全部</el-button>
    </div>
    <div class="panel">
      <el-skeleton :loading="loading" :throttle="300">
        <template #template>
          <div class="table-skeleton">
            <div v-for="i in 9" :key="i" class="table-skeleton-row skeleton-shimmer"></div>
          </div>
        </template>
        <el-table :data="items" border height="560" @sort-change="handleSortChange" @row-click="openDetail" @selection-change="handleSelectionChange">
        <el-table-column v-if="canManage" type="selection" width="45" fixed="left" />
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
        >
          <template v-if="column.key === 'assetType'" #default="{ row }">
            <el-tag :type="dictItem(assetTypeMap, row.assetType).type" size="small" effect="plain">
              {{ dictItem(assetTypeMap, row.assetType).label }}
            </el-tag>
          </template>
          <template v-else #default="{ row }">{{ row[column.prop] }}</template>
        </el-table-column>
        <el-table-column label="维保到期" width="110" fixed="right">
          <template #default="{ row }">
            <el-tag v-if="warrantyStatus(row)" :type="warrantyStatus(row).type" size="small" effect="plain">{{ warrantyStatus(row).text }}</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
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
      </el-skeleton>
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
          <el-col :span="8"><el-form-item label="关联IP"><el-input v-model="form.additionalIPs" placeholder="多网卡 IP，逗号分隔" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="厂商"><el-input v-model="form.manufacturer" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="资产类型"><el-input v-model="form.assetType" placeholder="服务器/网络设备/数据库" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="操作系统"><el-input v-model="form.os" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="资产归属"><el-input v-model="form.owner" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="所在网段"><el-input v-model="form.subnet" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="机房"><el-select v-model="form.location" filterable allow-create clearable placeholder="选择/输入机房"><el-option v-for="room in roomOptions" :key="room.name" :label="room.name" :value="room.name" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="机柜"><el-select v-model="form.rack" filterable allow-create clearable placeholder="选择/输入机柜"><el-option v-for="rack in rackOptions" :key="rack.name" :label="rack.name" :value="rack.name" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="U位"><el-input v-model="form.rackPosition" placeholder="如 12 或 12-14" /></el-form-item></el-col>
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

    <!-- 批量编辑弹窗 -->
    <el-dialog v-model="batchEditVisible" title="批量编辑资产" width="620px" class="batch-edit-dialog">
      <el-alert :title="`将对选中的 ${selectedIds.length} 台资产生效：仅勾选并填写了值的字段会被修改`" type="info" :closable="false" show-icon style="margin-bottom: 14px" />
      <el-form label-width="130px" label-position="left">
        <el-form-item v-for="field in batchEditFields" :key="field.key" :label="field.label">
          <div class="batch-edit-row">
            <el-checkbox v-model="batchEditForm[field.key].checked" style="margin-right: 10px" />
            <el-select
              v-if="field.options"
              v-model="batchEditForm[field.key].value"
              :disabled="!batchEditForm[field.key].checked"
              placeholder="选择状态"
              clearable
              style="flex: 1"
            >
              <el-option v-for="opt in field.options" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <el-input
              v-else
              v-model="batchEditForm[field.key].value"
              :disabled="!batchEditForm[field.key].checked"
              :placeholder="`填写新的${field.label}`"
              clearable
              style="flex: 1"
            />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchEditVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchEditSubmitting" @click="submitBatchEdit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 资产详情弹窗（弹出式 + 动画） -->
    <AssetDetailDialog v-model="detail.visible" :asset="detail.asset" @updated="load" />
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Download, EditPen, Plus, Printer, Refresh, Search, Setting, Upload } from '@element-plus/icons-vue'
import { assetsApi } from '../api'
import { rackApi } from '../api'
import AssetDetailDialog from '../components/assets/AssetDetailDialog.vue'
import ConfirmAction from '../components/common/ConfirmAction.vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import PageHeader from '../components/common/PageHeader.vue'
import { useAuthStore } from '../stores/auth'
import { canManageAssets } from '../utils/permissions'
import {
  ONLINE_STATUS_MAP,
  ASSET_TYPE_MAP,
  ASSET_STATUS_MAP,
  dictItem,
  dictOptions
} from '../constants/dictionaries'

const auth = useAuthStore()
const canManage = computed(() => canManageAssets(auth.user))
const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive(emptyAsset())
const roomOptions = ref([])
const rackOptions = ref([])
const allRacks = ref([])
const filters = reactive({
  q: '',
  assetType: '',
  subnet: '',
  owner: '',
  manufacturer: '',
  openPort: '',
  service: '',
  onlineStatus: '',
  rack: ''
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

// 筛选字段配置：component=input（文本输入）或 select（下拉）
const filterFields = [
  { key: 'q', label: '关键词搜索', component: 'input', placeholder: '搜索 IP、主机名、MAC、端口、服务、负责人、网段', width: 250, default: true },
  { key: 'onlineStatus', label: '在线状态', component: 'select', placeholder: '在线状态', width: 130, default: true },
  { key: 'assetType', label: '资产类型', component: 'input', placeholder: '资产类型', width: 130, default: false },
  { key: 'subnet', label: '所在网段', component: 'input', placeholder: '所在网段', width: 130, default: false },
  { key: 'owner', label: '负责人', component: 'input', placeholder: '负责人', width: 120, default: false },
  { key: 'openPort', label: '端口', component: 'input', placeholder: '端口', width: 110, default: false },
  { key: 'service', label: '服务/应用', component: 'input', placeholder: '服务/应用', width: 130, default: false }
]
const filterFieldStorageKey = 'asset.filterFields'
const filterFieldKeys = ref(loadFilterFieldKeys())
const visibleFilterFields = computed(() => filterFields.filter((field) => filterFieldKeys.value.includes(field.key)))
const columnStorageKey = 'asset.visibleColumns'
const visibleColumnKeys = ref(loadVisibleColumnKeys())
const visibleColumns = computed(() => columns.filter((column) => visibleColumnKeys.value.includes(column.key)))
const topAssetTypeText = computed(() => {
  const top = stats.byAssetType?.[0]
  return top ? `${top.label || '未填'} ${top.count}` : '-'
})

const onlineStatusMap = ONLINE_STATUS_MAP
const assetTypeMap = ASSET_TYPE_MAP
const onlineStatusOptions = dictOptions(ONLINE_STATUS_MAP)

// warrantyStatus 计算维保到期状态：已过期/30 天内到期高亮。
function warrantyStatus(row) {
  if (!row.warrantyExpireDate) return null
  const expire = new Date(row.warrantyExpireDate).getTime()
  const diff = expire - Date.now()
  if (diff < 0) return { text: '已过期', type: 'danger' }
  if (diff <= 30 * 86400000) return { text: '即将到期', type: 'warning' }
  return null
}

const detail = reactive({ visible: false, asset: null })
const route = useRoute()
const router = useRouter()

async function openDetail(row) {
  detail.asset = row
  detail.visible = true
}

// 扫码落地：URL 带 assetId 时自动打开对应资产详情。
async function openAssetFromQuery() {
  const assetId = route.query.assetId
  if (!assetId) return
  try {
    const asset = await assetsApi.detail(assetId)
    openDetail(asset)
  } catch {
    ElMessage.warning('未找到对应资产')
  }
}

// 打印选中资产的标签。
function printLabels() {
  if (!selectedIds.value.length) return ElMessage.warning('请先勾选资产')
  const url = router.resolve({ path: '/asset-labels', query: { ids: selectedIds.value.join(',') } }).href
  window.open(url, '_blank')
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
    additionalIPs: '',
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

// loadRackOptions 加载机房与机柜选项（用于资产表单的机房/机柜下拉）。
async function loadRackOptions() {
  try {
    const data = await rackApi.listRooms()
    roomOptions.value = data.items || []
    const racks = []
    for (const room of roomOptions.value) {
      if (room.racks?.length) {
        for (const r of room.racks) racks.push({ name: r.name, roomName: room.name })
      }
    }
    allRacks.value = racks
    updateRackOptions()
  } catch {
    roomOptions.value = []
  }
}

function updateRackOptions() {
  if (form.location) {
    rackOptions.value = allRacks.value.filter((r) => r.roomName === form.location)
  } else {
    rackOptions.value = allRacks.value
  }
}

watch(() => form.location, updateRackOptions)

async function load() {  loading.value = true
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
  try {
    await ElMessageBox.confirm(`确认删除资产 ${row.ip}？`, '删除确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await assetsApi.remove(row.id)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

const selectedIds = ref([])
function handleSelectionChange(rows) {
  selectedIds.value = rows.map((row) => row.id)
}

async function batchDelete() {
  const count = selectedIds.value.length
  if (!count) return
  // 二次确认：输入「删除N台」才执行
  try {
    const { value } = await ElMessageBox.prompt(
      `确认删除选中的 ${count} 台资产？此操作不可恢复。\n请输入「删除${count}台」以确认：`,
      '批量删除确认',
      {
        type: 'warning',
        inputPlaceholder: `删除${count}台`,
        confirmButtonText: '确认删除',
        cancelButtonText: '取消'
      }
    )
    if (String(value || '').trim() !== `删除${count}台`) {
      ElMessage.warning('确认文字不匹配，已取消')
      return
    }
  } catch {
    return
  }
  try {
    const res = await assetsApi.batchDelete(selectedIds.value)
    ElMessage.success(`已删除 ${res.deleted || count} 台资产`)
    selectedIds.value = []
    await load()
  } catch (e) {
    ElMessage.error(e.message || '批量删除失败')
  }
}

// ---------- 批量编辑 ----------
const batchEditVisible = ref(false)
const batchEditSubmitting = ref(false)
const batchEditFields = [
  { key: 'owner', label: '资产归属/负责人' },
  { key: 'department', label: '部门' },
  { key: 'location', label: '机房' },
  { key: 'rack', label: '机柜' },
  { key: 'rackPosition', label: 'U 位' },
  { key: 'environment', label: '环境' },
  { key: 'businessSystem', label: '业务系统' },
  { key: 'maintenanceVendor', label: '维保厂商' },
  {
    key: 'status',
    label: '资产状态',
    options: Object.entries(ASSET_STATUS_MAP).map(([value, item]) => ({ value, label: item.label }))
  },
  { key: 'remark', label: '备注' }
]
const batchEditForm = reactive({})
batchEditFields.forEach((field) => {
  batchEditForm[field.key] = { checked: false, value: '' }
})

function openBatchEdit() {
  if (!selectedIds.value.length) return ElMessage.warning('请先勾选资产')
  batchEditFields.forEach((field) => {
    batchEditForm[field.key].checked = false
    batchEditForm[field.key].value = ''
  })
  batchEditVisible.value = true
}

async function submitBatchEdit() {
  const fields = {}
  batchEditFields.forEach((field) => {
    if (batchEditForm[field.key].checked && String(batchEditForm[field.key].value || '').trim() !== '') {
      fields[field.key] = batchEditForm[field.key].value
    }
  })
  if (!Object.keys(fields).length) {
    ElMessage.warning('请至少勾选并填写一个字段')
    return
  }
  batchEditSubmitting.value = true
  try {
    const res = await assetsApi.batchUpdate(selectedIds.value, fields)
    ElMessage.success(`已更新 ${res.updated || selectedIds.value.length} 台资产`)
    batchEditVisible.value = false
    selectedIds.value = []
    await load()
  } catch (e) {
    ElMessage.error(e.message || '批量编辑失败')
  } finally {
    batchEditSubmitting.value = false
  }
}

// 当前生效筛选条件（chips 展示，让筛选状态一目了然）
const activeFilterChips = computed(() => {
  const labelMap = Object.fromEntries(filterFields.map((f) => [f.key, f.label]))
  return Object.entries(filters)
    .filter(([, value]) => String(value || '').trim() !== '')
    .map(([key, value]) => ({ key, label: labelMap[key] || key, value }))
})

function clearFilter(key) {
  filters[key] = ''
  applyFilters()
}

async function downloadTemplate() {
  const data = await assetsApi.template({ format: 'xlsx' })
  saveBlob(data, 'asset-import-template.xlsx')
}

async function exportAssets() {
  const data = await assetsApi.export(buildQuery(false))
  saveBlob(data, 'assets-export.csv')
}

async function exportStatsReport() {
  try {
    const data = await assetsApi.exportStatsReport()
    saveBlob(data, 'asset-stats-report.csv')
  } catch (e) {
    ElMessage.error(e.message || '导出报表失败')
  }
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

function loadFilterFieldKeys() {
  const defaults = filterFields.filter((field) => field.default).map((field) => field.key)
  try {
    const saved = JSON.parse(localStorage.getItem(filterFieldStorageKey) || '[]')
    if (!Array.isArray(saved) || saved.length === 0) return defaults
    return [...new Set(saved)].filter((key) => filterFields.some((field) => field.key === key))
  } catch {
    return defaults
  }
}

watch(
  filterFieldKeys,
  (value) => {
    if (value.length === 0) {
      // 空选择保护：回退到默认关键词搜索字段（UI 不空白）
      filterFieldKeys.value = ['q']
      return
    }
    // 清除被隐藏字段的残留筛选值，避免“幽灵筛选”仍生效
    const removed = filterFields.filter((field) => !value.includes(field.key)).map((field) => field.key)
    const hadValue = removed.some((key) => String(filters[key] || '').trim() !== '')
    if (hadValue) {
      removed.forEach((key) => {
        filters[key] = ''
      })
      load()
    }
    localStorage.setItem(filterFieldStorageKey, JSON.stringify(value))
  },
  { deep: true }
)

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

onMounted(() => {
  loadRackOptions()
  openAssetFromQuery()
  // 机柜视图跳转：URL 带 rack 时按机柜名过滤资产列表
  if (route.query.rack) {
    filters.rack = String(route.query.rack)
  }
  load()
})
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

/* 表格骨架屏（配合 el-skeleton 自定义 template） */
.table-skeleton {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
}

.table-skeleton-row {
  height: 40px;
  border-radius: 8px;
}

/* 批量编辑弹窗：勾选框 + 输入框一行排布 */
.batch-edit-row {
  display: flex;
  align-items: center;
  width: 100%;
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  padding-top: 12px;
}

:deep(.toolbar-actions) {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  padding: 0;
  overflow: visible;
  position: relative;
  z-index: 10;
}

/* 外层滚动容器：容纳 hover 上浮与阴影，避免被上下内容裁剪/遮挡 */
.toolbar-scroll {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 6px 2px 14px;
  min-width: 0;
}

.filter-settings-title {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 8px;
}

.filter-chips {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
  padding: 8px 12px;
  background: var(--brand-gradient-soft);
  border: 1px solid rgba(99, 102, 241, 0.15);
  border-radius: var(--radius-md);
}

.filter-chips-label {
  font-size: 12px;
  color: var(--text-secondary);
  flex-shrink: 0;
}

@media (max-width: 900px) {
  .asset-stats {
    grid-template-columns: repeat(2, minmax(140px, 1fr));
  }
}
</style>
