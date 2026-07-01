<template>
  <section class="page">
    <PageHeader title="用户角色">
      <template #actions>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增用户</el-button>
      </template>
    </PageHeader>
    <div class="panel">
      <el-table :data="items" v-loading="loading">
        <el-table-column prop="username" label="账号" width="150" />
        <el-table-column prop="name" label="姓名" min-width="160" />
        <el-table-column prop="authSource" label="来源" width="100">
          <template #default="{ row }">
            <StatusTag :value="row.authSource" :map="AUTH_SOURCE_MAP" />
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="department" label="部门" width="140" />
        <el-table-column prop="role" label="角色" width="160">
          <template #default="{ row }">
            <RoleTag :value="row.role" />
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <StatusTag :value="row.status" :map="USER_STATUS_MAP" />
          </template>
        </el-table-column>
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

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑用户' : '新增用户'" width="520px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="账号"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role">
            <el-option v-for="option in roleOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态"><el-select v-model="form.status"><el-option v-for="option in userStatusOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
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
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { adApi, usersApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'
import RoleTag from '../components/common/RoleTag.vue'
import StatusTag from '../components/common/StatusTag.vue'
import { AUTH_SOURCE_MAP, ROLE_MAP, USER_STATUS_MAP, dictOptions } from '../constants/dictionaries'

const items = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive(emptyUser())
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
const roleOptions = dictOptions(ROLE_MAP)
const userStatusOptions = dictOptions(USER_STATUS_MAP)

function emptyUser() {
  return { username: '', name: '', role: 'applicant', status: 'active', password: '', authSource: 'local', email: '', department: '' }
}

async function load() {
  loading.value = true
  try {
    const data = await usersApi.list()
    items.value = data.items
    await loadADConfig()
  } finally {
    loading.value = false
  }
}

async function loadADConfig() {
  const data = await adApi.config()
  Object.assign(adConfig, data, { bindPassword: '' })
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
  if (form.id) await usersApi.update(form.id, form)
  else await usersApi.create(form)
  ElMessage.success('已保存')
  dialogVisible.value = false
  load()
}

async function saveADConfig() {
  await adApi.saveConfig(adConfig)
  ElMessage.success('AD 配置已保存')
  await loadADConfig()
}

async function testAD() {
  try {
    await adApi.test()
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
    lookupResult.value = await adApi.lookupUser({ username: lookupUsername.value })
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '查询失败')
  }
}

async function importADUser() {
  await adApi.importUser({ username: lookupResult.value.username, role: importRole.value, status: 'active' })
  ElMessage.success('AD 用户已导入')
  lookupResult.value = null
  lookupUsername.value = ''
  await load()
}

onMounted(load)
</script>

<style scoped>
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
