<template>
  <section class="page">
    <div class="toolbar">
      <h2>用户角色</h2>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增用户</el-button>
    </div>
    <div class="panel">
      <el-table :data="items" v-loading="loading">
        <el-table-column prop="username" label="账号" width="150" />
        <el-table-column prop="name" label="姓名" min-width="160" />
        <el-table-column prop="authSource" label="来源" width="100" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="department" label="部门" width="140" />
        <el-table-column prop="role" label="角色" width="160" />
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="panel ad-panel">
      <div class="toolbar">
        <h3>AD 域控配置</h3>
        <div class="toolbar-actions">
          <el-button @click="testAD">测试连接</el-button>
          <el-button type="primary" @click="saveADConfig">保存配置</el-button>
        </div>
      </div>
      <el-form :model="adConfig" label-width="110px">
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="启用"><el-switch v-model="adConfig.enabled" /></el-form-item></el-col>
          <el-col :span="16"><el-form-item label="LDAP地址"><el-input v-model="adConfig.ldapUrl" placeholder="ldap://dc.example.com:389" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Base DN"><el-input v-model="adConfig.baseDn" placeholder="dc=example,dc=com" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Bind DN"><el-input v-model="adConfig.bindDn" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Bind密码"><el-input v-model="adConfig.bindPassword" type="password" show-password :placeholder="adConfig.hasBindPassword ? '留空表示不修改' : '首次保存必填'" /></el-form-item></el-col>
          <el-col :span="12">
            <el-form-item label="登录名格式">
              <el-select v-model="adConfig.loginAttribute">
                <el-option label="域账号短名" value="sAMAccountName" />
                <el-option label="邮箱/UPN" value="userPrincipalName" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12"><el-form-item label="只查用户"><el-switch v-model="adConfig.filterUserObject" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="排除禁用"><el-switch v-model="adConfig.excludeDisabled" /></el-form-item></el-col>
        </el-row>
        <el-collapse>
          <el-collapse-item title="高级过滤器" name="filter">
            <el-form-item label="启用高级">
              <el-switch v-model="adConfig.advancedFilter" />
            </el-form-item>
            <el-form-item label="过滤器">
              <el-input v-model="adConfig.userFilter" :disabled="!adConfig.advancedFilter" />
            </el-form-item>
          </el-collapse-item>
        </el-collapse>
      </el-form>
      <el-divider>按域账号导入用户</el-divider>
      <div class="ad-import">
        <el-input v-model="lookupUsername" placeholder="输入 sAMAccountName" />
        <el-button @click="lookupADUser">查询</el-button>
      </div>
      <el-descriptions v-if="lookupResult" class="lookup-result" :column="1" border>
        <el-descriptions-item label="账号">{{ lookupResult.username }}</el-descriptions-item>
        <el-descriptions-item label="姓名">{{ lookupResult.displayName }}</el-descriptions-item>
        <el-descriptions-item label="邮箱">{{ lookupResult.email || '-' }}</el-descriptions-item>
        <el-descriptions-item label="部门">{{ lookupResult.department || '-' }}</el-descriptions-item>
        <el-descriptions-item label="DN">{{ lookupResult.dn }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="lookupResult" class="ad-import-actions">
        <el-select v-model="importRole">
          <el-option label="申请人" value="applicant" />
          <el-option label="审批人" value="approver" />
          <el-option label="资产管理员" value="asset_manager" />
          <el-option label="管理员" value="admin" />
        </el-select>
        <el-button type="primary" @click="importADUser">导入用户</el-button>
      </div>
    </div>

    <div class="panel approver-panel">
      <div class="toolbar">
        <h3>工单类型审批人配置</h3>
      </div>
      <el-table :data="ticketTypes">
        <el-table-column prop="label" label="工单类型" min-width="180" />
        <el-table-column label="默认审批人" min-width="220">
          <template #default="{ row }">
            <el-select v-model="approverMap[row.value]" placeholder="选择审批人">
              <el-option v-for="user in approverUsers" :key="user.id" :label="`${user.name} (${user.username})`" :value="user.id" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button type="primary" link @click="saveApprover(row.value)">保存</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑用户' : '新增用户'" width="520px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="账号"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role">
            <el-option label="管理员" value="admin" />
            <el-option label="资产管理员" value="asset_manager" />
            <el-option label="审批人" value="approver" />
            <el-option label="申请人" value="applicant" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态"><el-select v-model="form.status"><el-option label="启用" value="active" /><el-option label="禁用" value="disabled" /></el-select></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item label="部门"><el-input v-model="form.department" /></el-form-item>
        <el-form-item v-if="form.authSource !== 'ad'" label="密码"><el-input v-model="form.password" type="password" show-password placeholder="编辑时留空则不修改" /></el-form-item>
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
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { api } from '../api'

const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive(emptyUser())
const approverMap = reactive({})
const adConfig = reactive({
  enabled: false,
  ldapUrl: '',
  baseDn: '',
  bindDn: '',
  bindPassword: '',
  loginAttribute: 'sAMAccountName',
  filterUserObject: true,
  excludeDisabled: true,
  advancedFilter: false,
  userFilter: '(&(objectClass=user)(sAMAccountName=%s)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))',
  hasBindPassword: false
})
const lookupUsername = ref('')
const lookupResult = ref(null)
const importRole = ref('applicant')
const ticketTypes = [
  { label: '资产登记', value: 'asset_register' },
  { label: '资产变更', value: 'asset_change' },
  { label: '资产下线/报废', value: 'asset_retire' },
  { label: '权限/维护申请', value: 'maintenance' }
]
const approverUsers = computed(() => items.value.filter((user) => ['admin', 'approver'].includes(user.role) && user.status === 'active'))

function emptyUser() {
  return { username: '', name: '', role: 'applicant', status: 'active', password: '', authSource: 'local', email: '', department: '' }
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/users')
    items.value = data.items
    await loadApprovers()
    await loadADConfig()
  } finally {
    loading.value = false
  }
}

async function loadADConfig() {
  const { data } = await api.get('/ad/config')
  Object.assign(adConfig, data, { bindPassword: '' })
}

async function loadApprovers() {
  const { data } = await api.get('/ticket-type-approvers')
  for (const item of data.items) {
    approverMap[item.type] = item.approverId
  }
}

function openCreate() {
  Object.assign(form, emptyUser(), { id: undefined })
  dialogVisible.value = true
}

function openEdit(row) {
  Object.assign(form, row, { password: '' })
  dialogVisible.value = true
}

async function save() {
  if (form.id) await api.put(`/users/${form.id}`, form)
  else await api.post('/users', form)
  ElMessage.success('已保存')
  dialogVisible.value = false
  load()
}

async function saveApprover(type) {
  if (!approverMap[type]) {
    ElMessage.warning('请选择审批人')
    return
  }
  await api.put(`/ticket-type-approvers/${type}`, { approverId: approverMap[type] })
  ElMessage.success('审批人配置已保存')
  await loadApprovers()
}

async function saveADConfig() {
  await api.put('/ad/config', adConfig)
  ElMessage.success('AD 配置已保存')
  await loadADConfig()
}

async function testAD() {
  try {
    await api.post('/ad/test')
    ElMessage.success('AD 连接测试成功')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'AD 连接测试失败')
  }
}

async function lookupADUser() {
  if (!lookupUsername.value.trim()) {
    ElMessage.warning('请输入域账号')
    return
  }
  try {
    const { data } = await api.post('/ad/lookup-user', { username: lookupUsername.value })
    lookupResult.value = data
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '查询失败')
  }
}

async function importADUser() {
  await api.post('/ad/import-user', { username: lookupResult.value.username, role: importRole.value, status: 'active' })
  ElMessage.success('AD 用户已导入')
  lookupResult.value = null
  lookupUsername.value = ''
  await load()
}

onMounted(load)
</script>

<style scoped>
.approver-panel,
.ad-panel {
  margin-top: 16px;
}

.ad-import,
.ad-import-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.lookup-result,
.ad-import-actions {
  margin-top: 12px;
}
</style>
