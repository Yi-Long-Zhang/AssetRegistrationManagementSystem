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

      <el-tab-pane label="IM 通知" name="im">
        <div class="settings-block">
          <h3>群机器人通知（钉钉 / 企业微信 / 飞书）</h3>
          <p class="desc">工单创建、审批通过/驳回、验收关闭等事件推送到群机器人；支持钉钉加签。</p>
          <el-form :model="imConfig" label-width="110px">
            <el-row>
              <el-col :span="8"><el-form-item label="启用"><el-switch v-model="imConfig.enabled" /></el-form-item></el-col>
              <el-col :span="16">
                <el-form-item label="平台">
                  <el-select v-model="imConfig.platform" style="width: 100%">
                    <el-option label="钉钉" value="dingtalk" />
                    <el-option label="企业微信" value="wecom" />
                    <el-option label="飞书" value="feishu" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="24"><el-form-item label="Webhook"><el-input v-model="imConfig.webhook" placeholder="https://oapi.dingtalk.com/robot/send?access_token=xxx" /></el-form-item></el-col>
              <el-col :span="24"><el-form-item label="加签密钥"><el-input v-model="imConfig.secret" placeholder="钉钉安全设置加签密钥（可选）" /></el-form-item></el-col>
            </el-row>
            <el-form-item>
              <el-button @click="testIM">发送测试</el-button>
              <el-button type="primary" @click="saveIMConfig">保存配置</el-button>
            </el-form-item>
          </el-form>
        </div>

        <div class="settings-block">
          <h3>回调验签配置（自建应用，档位 2）</h3>
          <p class="desc">IM 内按钮审批需平台自建应用 + 公网回调地址（POST /api/v1/im/callback）。配置对应平台密钥后即可校验回调签名。</p>
          <el-form :model="imCallbackConfig" label-width="120px">
            <el-row>
              <el-col :span="8"><el-form-item label="启用"><el-switch v-model="imCallbackConfig.enabled" /></el-form-item></el-col>
              <el-col :span="16">
                <el-form-item label="平台">
                  <el-select v-model="imCallbackConfig.platform" style="width: 100%">
                    <el-option label="钉钉" value="dingtalk" />
                    <el-option label="企业微信" value="wecom" />
                    <el-option label="飞书" value="feishu" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="12"><el-form-item label="AppSecret"><el-input v-model="imCallbackConfig.appSecret" placeholder="钉钉 AppSecret / 飞书应用 secret（留空不修改）" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="CorpID"><el-input v-model="imCallbackConfig.corpId" placeholder="企业微信 CorpID" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="Token"><el-input v-model="imCallbackConfig.token" placeholder="企微 Token / 飞书 verification token（留空不修改）" /></el-form-item></el-col>
              <el-col :span="12"><el-form-item label="EncodingAESKey"><el-input v-model="imCallbackConfig.encodingAESKey" placeholder="企业微信 EncodingAESKey（留空不修改）" /></el-form-item></el-col>
            </el-row>
            <el-form-item>
              <el-button type="primary" @click="saveIMCallbackConfig">保存回调配置</el-button>
            </el-form-item>
          </el-form>
        </div>

        <div class="settings-block">
          <h3>IM 用户绑定（回调鉴权基础）</h3>
          <p class="desc">建立 IM 用户与系统用户映射，为后续自建应用交互审批提供鉴权基础。</p>
          <div class="inline-form">
            <el-select v-model="bindingForm.userId" placeholder="选择系统用户" style="width: 180px" filterable>
              <el-option v-for="u in userOptions" :key="u.id" :label="`${u.username} (${u.realName || '-'})`" :value="u.id" />
            </el-select>
            <el-select v-model="bindingForm.platform" style="width: 120px">
              <el-option label="钉钉" value="dingtalk" />
              <el-option label="企业微信" value="wecom" />
              <el-option label="飞书" value="feishu" />
            </el-select>
            <el-input v-model="bindingForm.imUserId" placeholder="IM 用户标识（openId/userId）" style="width: 240px" />
            <el-button type="primary" @click="saveBinding">保存绑定</el-button>
          </div>
          <el-table :data="imBindings" size="small" border stripe style="margin-top: 12px">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column label="系统用户" min-width="140">
              <template #default="{ row }">{{ row.user?.username || '-' }}</template>
            </el-table-column>
            <el-table-column label="平台" width="110">
              <template #default="{ row }">{{ platformLabel(row.platform) }}</template>
            </el-table-column>
            <el-table-column prop="imUserId" label="IM 用户标识" min-width="180" />
            <el-table-column label="操作" width="90" align="center">
              <template #default="{ row }">
                <el-button size="small" type="danger" text @click="deleteBinding(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adApi, settingsApi } from '../api'
import { usersApi } from '../api/users'
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
const imConfig = reactive({
  enabled: false,
  platform: 'dingtalk',
  webhook: '',
  secret: ''
})
const imCallbackConfig = reactive({
  enabled: false,
  platform: 'dingtalk',
  appSecret: '',
  corpId: '',
  token: '',
  encodingAESKey: ''
})
const imBindings = ref([])
const userOptions = ref([])
const bindingForm = reactive({ userId: null, platform: 'dingtalk', imUserId: '' })
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

async function loadIMConfig() {
  try {
    const data = await settingsApi.imConfig()
    Object.assign(imConfig, {
      enabled: data.enabled || false,
      platform: data.platform || 'dingtalk',
      webhook: data.webhook || '',
      secret: data.secret || ''
    })
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '加载 IM 配置失败')
  }
}

async function saveIMConfig() {
  try {
    await settingsApi.saveIMConfig(imConfig)
    ElMessage.success('IM 配置已保存')
    await loadIMConfig()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存 IM 配置失败')
  }
}

async function loadIMCallbackConfig() {
  try {
    const data = await settingsApi.imCallbackConfig()
    Object.assign(imCallbackConfig, {
      enabled: data.enabled || false,
      platform: data.platform || 'dingtalk',
      corpId: data.corpId || '',
      appSecret: '',
      token: '',
      encodingAESKey: ''
    })
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '加载回调配置失败')
  }
}

async function saveIMCallbackConfig() {
  try {
    await settingsApi.saveIMCallbackConfig(imCallbackConfig)
    ElMessage.success('回调配置已保存')
    await loadIMCallbackConfig()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存回调配置失败')
  }
}

async function testIM() {
  try {
    await settingsApi.testIMConfig()
    ElMessage.success('测试消息已发送，请查看群机器人')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '发送测试失败')
  }
}

function platformLabel(platform) {
  return { dingtalk: '钉钉', wecom: '企业微信', feishu: '飞书' }[platform] || platform
}

async function loadIMBindings() {
  try {
    imBindings.value = await settingsApi.imBindings()
    if (!userOptions.value.length) {
      userOptions.value = await usersApi.list()
    }
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '加载 IM 绑定失败')
  }
}

async function saveBinding() {
  if (!bindingForm.userId || !bindingForm.imUserId.trim()) {
    ElMessage.warning('请选择系统用户并填写 IM 用户标识')
    return
  }
  try {
    await settingsApi.saveIMBinding(bindingForm)
    ElMessage.success('绑定已保存')
    bindingForm.userId = null
    bindingForm.imUserId = ''
    await loadIMBindings()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存绑定失败')
  }
}

async function deleteBinding(row) {
  try {
    await settingsApi.deleteIMBinding(row.userId)
    ElMessage.success('已删除绑定')
    await loadIMBindings()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '删除绑定失败')
  }
}

onMounted(async () => {
  await Promise.all([loadADConfig(), loadMailConfig(), loadIMConfig(), loadIMCallbackConfig(), loadIMBindings()])
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
