<template>
  <section class="page">
    <PageHeader title="工单流程">
      <template #actions>
        <el-button :disabled="!selectedArchiveRows.length" @click="downloadSelectedArchives">批量下载归档</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建工单</el-button>
      </template>
    </PageHeader>
    <div class="panel">
      <el-tabs v-model="activeView" @tab-change="load">
        <el-tab-pane label="我的待办" name="todo" />
        <el-tab-pane label="我提交的" name="submitted" />
        <el-tab-pane label="全部" name="all" />
      </el-tabs>
      <el-skeleton :loading="loading" :throttle="300">
        <template #template>
          <div class="table-skeleton">
            <div v-for="i in 8" :key="i" class="table-skeleton-row skeleton-shimmer"></div>
          </div>
        </template>
        <el-table :data="items" @selection-change="selectedRows = $event">
        <el-table-column type="selection" width="46" :selectable="canSelectArchive" />
        <el-table-column prop="id" label="编号" width="80" />
        <el-table-column prop="title" label="标题" min-width="180" />
        <el-table-column prop="type" label="类型" width="150">
          <template #default="{ row }">
            <StatusTag :value="row.type" :map="TICKET_TYPE_MAP" />
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <StatusTag :value="row.status" :map="TICKET_STATUS_MAP" />
          </template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="100">
          <template #default="{ row }">
            <StatusTag :value="row.priority" :map="PRIORITY_MAP" />
          </template>
        </el-table-column>
        <el-table-column label="申请人" width="120"><template #default="{ row }">{{ row.applicant?.name }}</template></el-table-column>
        <el-table-column prop="currentWorkflowStepName" label="当前节点" width="160" />
        <el-table-column label="SLA" width="130">
          <template #default="{ row }">
            <span v-if="slaInfo(row)" :class="slaInfo(row).overdue ? 'sla-overdue' : 'sla-ok'">{{ slaInfo(row).text }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="360">
          <template #default="{ row }">
            <el-button link type="primary" @click="view(row)">详情</el-button>
            <el-button v-for="action in actions(row)" :key="action.name" link type="primary" @click="doAction(row, action.name)">{{ action.label }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      </el-skeleton>
    </div>

    <el-dialog v-model="dialogVisible" title="新建工单" width="640px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="类型"><el-select v-model="form.type"><el-option v-for="option in ticketTypeOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="关联资产">
          <el-select v-model="form.assetIds" multiple filterable clearable placeholder="选择受影响资产（可多选）" style="width: 100%">
            <el-option v-for="asset in assetOptions" :key="asset.id" :label="`${asset.hostname || asset.assetNo} (${asset.ip})`" :value="asset.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="优先级"><el-select v-model="form.priority"><el-option v-for="option in priorityOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="设备类型"><el-input v-model="form.deviceType" placeholder="服务器/交换机/防火墙" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="设备名称"><el-input v-model="form.deviceName" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="IP地址"><el-input v-model="form.ipAddress" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="开放端口"><el-input v-model="form.openPorts" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="运行服务"><el-input v-model="form.runningServices" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="应用版本"><el-input v-model="form.appVersion" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="厂商"><el-input v-model="form.manufacturer" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="防病毒"><el-input v-model="form.antivirus" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="申请原因"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="变更内容"><el-input v-model="form.changeContent" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="影响评估"><el-input v-model="form.impact" type="textarea" :rows="2" placeholder="无 / 说明影响范围" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存草稿</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="drawerVisible" title="工单详情" size="520px">
      <template v-if="detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="标题">{{ detail.title }}</el-descriptions-item>
          <el-descriptions-item label="状态"><StatusTag :value="detail.status" :map="TICKET_STATUS_MAP" /></el-descriptions-item>
          <el-descriptions-item label="当前节点">{{ detail.currentWorkflowStepName || '-' }}</el-descriptions-item>
          <el-descriptions-item v-if="slaInfo(detail)" label="SLA 时效">
            <span :class="slaInfo(detail).overdue ? 'sla-overdue' : 'sla-ok'">{{ slaInfo(detail).text }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="设备">{{ detail.deviceType || '-' }} / {{ detail.deviceName || '-' }} / {{ detail.ipAddress || '-' }}</el-descriptions-item>
          <el-descriptions-item label="关联资产">
            <template v-if="detail.assets?.length">
              <div v-for="link in detail.assets" :key="link.id" class="linked-asset">
                {{ link.asset?.hostname || link.asset?.assetNo || '资产#' + link.assetId }} ({{ link.asset?.ip || '-' }})
              </div>
            </template>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="服务信息">{{ detail.openPorts || '-' }} / {{ detail.runningServices || '-' }} / {{ detail.appVersion || '-' }}</el-descriptions-item>
          <el-descriptions-item label="厂商/防病毒">{{ detail.manufacturer || '-' }} / {{ detail.antivirus || '-' }}</el-descriptions-item>
          <el-descriptions-item label="申请原因">{{ detail.description || '-' }}</el-descriptions-item>
          <el-descriptions-item label="变更内容">{{ detail.changeContent || '-' }}</el-descriptions-item>
          <el-descriptions-item label="影响评估">{{ detail.impact || '-' }}</el-descriptions-item>
          <el-descriptions-item label="执行记录">{{ detail.result || '-' }}</el-descriptions-item>
          <el-descriptions-item label="验收结果">{{ detail.acceptanceResult || '-' }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.archiveNo" label="归档编号">{{ detail.archiveNo }} {{ detail.archivedAt || '' }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="detail.status === 'closed'" class="archive-actions">
          <el-button type="primary" @click="downloadArchive">下载归档 PDF</el-button>
        </div>
        <el-steps v-if="detail.workflowSteps?.length" class="workflow-steps" :active="activeStepIndex" finish-status="success" process-status="process">
          <el-step v-for="step in detail.workflowSteps" :key="step.id" :title="step.name" :description="stepDescription(step)" />
        </el-steps>
        <el-timeline class="timeline">
          <el-timeline-item v-for="record in detail.records" :key="record.id" :timestamp="record.createdAt">
            {{ record.actor?.name }} {{ record.action }}：{{ record.remark || '-' }}
          </el-timeline-item>
        </el-timeline>
        <el-divider>评论</el-divider>
        <div class="comments">
          <div v-for="comment in comments" :key="comment.id" class="comment">
            <strong>{{ comment.actor?.name }}</strong>
            <span class="muted">{{ comment.createdAt }}</span>
            <p>{{ comment.content }}</p>
          </div>
          <el-input v-model="commentText" type="textarea" :rows="3" placeholder="追加评论" />
          <el-button class="comment-submit" type="primary" @click="submitComment">发送评论</el-button>
        </div>
        <el-divider>附件</el-divider>
        <el-upload :http-request="uploadAttachment" :show-file-list="false">
          <el-button>上传附件</el-button>
        </el-upload>
        <el-table class="attachments" :data="attachments" size="small">
          <el-table-column prop="originalName" label="文件名" min-width="180" />
          <el-table-column prop="size" label="大小" width="100" />
          <el-table-column label="上传人" width="100"><template #default="{ row }">{{ row.uploader?.name }}</template></el-table-column>
          <el-table-column label="操作" width="90"><template #default="{ row }"><el-button link type="primary" @click="downloadAttachment(row)">下载</el-button></template></el-table-column>
        </el-table>
      </template>
    </el-drawer>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { ticketsApi } from '../api'
import { assetsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'
import StatusTag from '../components/common/StatusTag.vue'
import { PRIORITY_MAP, TICKET_STATUS_MAP, TICKET_TYPE_MAP, dictOptions } from '../constants/dictionaries'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const items = ref([])
const selectedRows = ref([])
const loading = ref(false)
const activeView = ref('todo')
const dialogVisible = ref(false)
const drawerVisible = ref(false)
const detail = ref(null)
const comments = ref([])
const attachments = ref([])
const commentText = ref('')
const form = reactive(emptyForm())
const ticketTypeOptions = dictOptions(TICKET_TYPE_MAP)
const priorityOptions = dictOptions(PRIORITY_MAP)
const assetOptions = ref([])

async function loadAssets() {
  try {
    const data = await assetsApi.list({ page: 1, pageSize: 200 })
    assetOptions.value = data.items || []
  } catch {
    assetOptions.value = []
  }
}

// slaInfo 计算工单 SLA 剩余/超时信息：审批阶段看审批截止，执行阶段看完成截止。
function slaInfo(row) {
  if (!row) return null
  const deadline =
    row.status === 'pending_approval'
      ? row.slaApprovalDeadline
      : row.status === 'in_progress'
        ? row.slaCompletionDeadline
        : null
  if (!deadline) return null
  const diff = new Date(deadline).getTime() - Date.now()
  const overdue = diff < 0
  const hours = Math.abs(diff) / 3600000
  let span
  if (hours >= 24) span = `${(hours / 24).toFixed(1)} 天`
  else if (hours >= 1) span = `${hours.toFixed(1)} 小时`
  else span = `${Math.max(1, Math.round(hours * 60))} 分钟`
  return { text: `${overdue ? '已超时 ' : '剩余 '}${span}`, overdue }
}

async function load() {
  loading.value = true
  try {
    const data = await ticketsApi.list({ view: activeView.value })
    items.value = data.items
    selectedRows.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}

function emptyForm() {
  return {
    type: 'asset_register',
    title: '',
    assetIds: [],
    priority: 'normal',
    description: '',
    deviceType: '',
    deviceName: '',
    ipAddress: '',
    openPorts: '',
    runningServices: '',
    appVersion: '',
    manufacturer: '',
    antivirus: '',
    changeContent: '',
    impact: '无',
    remark: ''
  }
}

async function save() {
  await ticketsApi.create(form)
  ElMessage.success('已创建草稿')
  dialogVisible.value = false
  load()
}

async function view(row) {
  const data = await ticketsApi.detail(row.id)
  detail.value = data
  comments.value = data.comments || []
  attachments.value = data.attachments || []
  commentText.value = ''
  await Promise.all([loadComments(), loadAttachments()])
  drawerVisible.value = true
}

async function loadComments() {
  if (!detail.value) return
  const data = await ticketsApi.comments(detail.value.id)
  comments.value = data.items
}

async function loadAttachments() {
  if (!detail.value) return
  const data = await ticketsApi.attachments(detail.value.id)
  attachments.value = data.items
}

function actions(row) {
  const user = auth.user
  const role = user?.role
  const items = []
  if (row.status === 'draft' && (role === 'admin' || row.applicantId === user?.id)) items.push({ name: 'submit', label: '提交' })
  if (row.status === 'rejected' && (role === 'admin' || row.applicantId === user?.id)) items.push({ name: 'submit', label: '重新提交' })
  if (row.status === 'pending_approval' && ['admin', 'approver'].includes(role)) items.push({ name: 'approve', label: '通过' }, { name: 'reject', label: '驳回' })
  if (row.status === 'approved' && ['admin', 'asset_manager'].includes(role)) items.push({ name: 'start', label: '开始' })
  if (row.status === 'in_progress' && ['admin', 'asset_manager'].includes(role)) items.push({ name: 'complete', label: '完成' })
  if (row.status === 'pending_acceptance' && ['admin', 'applicant'].includes(role)) items.push({ name: 'accept', label: '验收' })
  if (['draft', 'rejected'].includes(row.status) && (role === 'admin' || row.applicantId === user?.id)) items.push({ name: 'cancel', label: '取消' })
  return items
}

async function doAction(row, action) {
  try {
    const { value } = await ElMessageBox.prompt('处理备注', '工单操作', { inputType: 'textarea', inputValue: action })
    await ticketsApi.action(row.id, action, {
      remark: value,
      result: action === 'complete' ? value : '',
      acceptanceResult: action === 'accept' ? value : ''
    })
    ElMessage.success('操作成功')
    load()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(error.response?.data?.error || '操作失败')
  }
}

async function submitComment() {
  if (!detail.value || !commentText.value.trim()) return
  await ticketsApi.createComment(detail.value.id, { content: commentText.value })
  commentText.value = ''
  ElMessage.success('评论已发送')
  await loadComments()
}

async function uploadAttachment(options) {
  const formData = new FormData()
  formData.append('file', options.file)
  try {
    await ticketsApi.uploadAttachment(detail.value.id, formData)
    ElMessage.success('附件已上传')
    await loadAttachments()
    options.onSuccess()
  } catch (error) {
    options.onError(error)
    ElMessage.error(error.response?.data?.error || '上传失败')
  }
}

async function downloadAttachment(row) {
  const data = await ticketsApi.downloadAttachment(detail.value.id, row.id)
  const url = URL.createObjectURL(data)
  const link = document.createElement('a')
  link.href = url
  link.download = row.originalName
  link.click()
  URL.revokeObjectURL(url)
}

async function downloadArchive() {
  const data = await ticketsApi.downloadArchive(detail.value.id)
  const url = URL.createObjectURL(data)
  const link = document.createElement('a')
  link.href = url
  link.download = `${detail.value.archiveNo || `ticket-${detail.value.id}`}.pdf`
  link.click()
  URL.revokeObjectURL(url)
}

const selectedArchiveRows = computed(() => selectedRows.value.filter(canSelectArchive))

function canSelectArchive(row) {
  return row.status === 'closed' && !!row.archiveNo
}

async function downloadSelectedArchives() {
  if (!selectedArchiveRows.value.length) {
    ElMessage.warning('请选择已关闭且已生成归档的工单')
    return
  }
  const ids = selectedArchiveRows.value.map((row) => row.id)
  const data = await ticketsApi.downloadArchives(ids)
  const url = URL.createObjectURL(data)
  const link = document.createElement('a')
  link.href = url
  link.download = `ticket-archives-${new Date().toISOString().slice(0, 10)}.zip`
  link.click()
  URL.revokeObjectURL(url)
}

function stepDescription(step) {
  const users = (step.approvers || []).map((item) => item.user?.name || item.user?.username).filter(Boolean).join('、')
  return `${users || '-'} ${step.actedAt || ''}`
}

const activeStepIndex = computed(() => {
  if (!detail.value?.workflowSteps?.length) return 0
  const pending = detail.value.workflowSteps.findIndex((step) => step.status === 'pending')
  return pending === -1 ? detail.value.workflowSteps.length : pending
})

onMounted(() => {
  load()
  loadAssets()
})
</script>

<style scoped>
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

.sla-overdue {
  color: #f56c6c;
  font-weight: 600;
}

.sla-ok {
  color: #67c23a;
}

.linked-asset {
  line-height: 1.8;
}

.timeline {
  margin-top: 24px;
}

.comments {
  display: grid;
  gap: 12px;
}

.comment {
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 8px;
}

.comment p {
  margin: 6px 0 0;
}

.comment-submit {
  justify-self: end;
}

.attachments {
  margin-top: 12px;
}

.archive-actions,
.workflow-steps {
  margin-top: 16px;
}
</style>
