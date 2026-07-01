<template>
  <section class="page">
    <PageHeader title="系统配置" />

    <el-tabs v-model="activeTab" class="settings-tabs">
      <el-tab-pane label="AD 域控" name="ad">
        <div class="panel">
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
          <div class="inline-form">
            <el-input v-model="lookupUsername" placeholder="输入 sAMAccountName" />
            <el-button @click="lookupADUser">查询</el-button>
          </div>
          <el-descriptions v-if="lookupResult" class="result-box" :column="1" border>
            <el-descriptions-item label="账号">{{ lookupResult.username }}</el-descriptions-item>
            <el-descriptions-item label="姓名">{{ lookupResult.displayName }}</el-descriptions-item>
            <el-descriptions-item label="邮箱">{{ lookupResult.email || '-' }}</el-descriptions-item>
            <el-descriptions-item label="部门">{{ lookupResult.department || '-' }}</el-descriptions-item>
            <el-descriptions-item label="DN">{{ lookupResult.dn }}</el-descriptions-item>
          </el-descriptions>
          <div v-if="lookupResult" class="inline-form result-actions">
            <el-select v-model="importRole">
              <el-option label="申请人" value="applicant" />
              <el-option label="审批人" value="approver" />
              <el-option label="资产管理员" value="asset_manager" />
              <el-option label="管理员" value="admin" />
            </el-select>
            <el-button type="primary" @click="importADUser">导入用户</el-button>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="邮件通知" name="mail">
        <div class="panel">
          <div class="toolbar">
            <h3>SMTP 邮件通知</h3>
            <div class="toolbar-actions">
              <el-button @click="testMail">发送测试</el-button>
              <el-button type="primary" @click="saveMailConfig">保存配置</el-button>
            </div>
          </div>
          <el-form :model="mailConfig" label-width="110px">
            <el-row :gutter="12">
              <el-col :span="8"><el-form-item label="启用"><el-switch v-model="mailConfig.enabled" /></el-form-item></el-col>
              <el-col :span="16"><el-form-item label="SMTP地址"><el-input v-model="mailConfig.smtpHost" placeholder="smtp.example.com" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="端口"><el-input-number v-model="mailConfig.smtpPort" :min="1" :max="65535" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="SSL/TLS"><el-switch v-model="mailConfig.useTls" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="STARTTLS"><el-switch v-model="mailConfig.startTls" :disabled="mailConfig.useTls" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="用户名"><el-input v-model="mailConfig.username" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="密码"><el-input v-model="mailConfig.password" type="password" show-password :placeholder="mailConfig.hasPassword ? '留空表示不修改' : '无认证可留空'" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="发件邮箱"><el-input v-model="mailConfig.fromAddress" placeholder="asset-system@example.com" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="发件名称"><el-input v-model="mailConfig.fromName" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="测试收件人"><el-input v-model="testRecipient" placeholder="留空则发送到发件邮箱" /></el-form-item></el-col>
            </el-row>
          </el-form>
        </div>
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adApi, settingsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const activeTab = ref('ad')
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
const mailConfig = reactive({
  enabled: false,
  smtpHost: '',
  smtpPort: 25,
  username: '',
  password: '',
  fromAddress: '',
  fromName: '资产管理系统',
  useTls: false,
  startTls: true,
  hasPassword: false
})
const lookupUsername = ref('')
const lookupResult = ref(null)
const importRole = ref('applicant')
const testRecipient = ref('')

async function loadADConfig() {
  const data = await adApi.config()
  Object.assign(adConfig, data, { bindPassword: '' })
}

async function loadMailConfig() {
  const data = await settingsApi.mailConfig()
  Object.assign(mailConfig, data, { password: '' })
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
}

async function saveMailConfig() {
  await settingsApi.saveMailConfig(mailConfig)
  ElMessage.success('邮件配置已保存')
  await loadMailConfig()
}

async function testMail() {
  try {
    await settingsApi.testMailConfig({ recipient: testRecipient.value })
    ElMessage.success('测试邮件已发送')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '测试邮件发送失败')
  }
}

onMounted(async () => {
  await Promise.all([loadADConfig(), loadMailConfig()])
})
</script>

<style scoped>
.settings-tabs {
  background: transparent;
}

.inline-form,
.result-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.inline-form .el-input {
  max-width: 320px;
}

.result-box,
.result-actions {
  margin-top: 12px;
}
</style>
