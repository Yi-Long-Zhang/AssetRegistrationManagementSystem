<template>
  <div>
    <el-tabs v-model="activeTab">
      <!-- ============ 规则管理 ============ -->
      <el-tab-pane label="发现规则" name="rules">
        <div class="toolbar">
          <el-button type="primary" @click="openRuleDialog()">新建规则</el-button>
          <span class="hint">需先安装 nmap：Windows 运行 <code>scripts/setup-nmap.ps1</code>，Linux/macOS 运行 <code>scripts/setup-nmap.sh</code></span>
        </div>
        <el-table :data="rules" v-loading="loadingRules" border stripe>
          <el-table-column prop="name" label="规则名称" min-width="140" />
          <el-table-column prop="targets" label="扫描目标" min-width="180" show-overflow-tooltip />
          <el-table-column prop="ports" label="端口" width="110">
            <template #default="{ row }">{{ row.ports || '默认' }}</template>
          </el-table-column>
          <el-table-column label="服务识别" width="90" align="center">
            <template #default="{ row }">{{ row.serviceDetect ? '是' : '否' }}</template>
          </el-table-column>
          <el-table-column prop="intervalMinutes" label="间隔(分)" width="90" align="center" />
          <el-table-column label="自动纳管" width="90" align="center">
            <template #default="{ row }">{{ row.autoAdopt ? '是' : '否' }}</template>
          </el-table-column>
          <el-table-column label="启用" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最近运行" width="170">
            <template #default="{ row }">{{ formatTime(row.lastRunAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="260" fixed="right">
            <template #default="{ row }">
              <el-button size="small" @click="openRuleDialog(row)">编辑</el-button>
              <el-button size="small" type="primary" plain :loading="runningRuleId === row.id" @click="runRule(row)">立即运行</el-button>
              <el-button size="small" type="warning" plain :loading="testingRuleId === row.id" @click="testRule(row)">试跑</el-button>
              <el-button size="small" type="danger" plain @click="removeRule(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-dialog v-model="ruleDialog.visible" :title="ruleDialog.form.id ? '编辑规则' : '新建规则'" width="560px">
          <el-form :model="ruleDialog.form" label-width="110px">
            <el-form-item label="规则名称" required>
              <el-input v-model="ruleDialog.form.name" placeholder="如：办公网段资产发现" />
            </el-form-item>
            <el-form-item label="扫描目标" required>
              <el-input v-model="ruleDialog.form.targets" type="textarea" :rows="2" placeholder="IP 或 CIDR，逗号分隔，如 192.168.1.0/24, 10.0.0.5" />
            </el-form-item>
            <el-form-item label="端口组">
              <el-select v-model="ruleDialog.form.portGroup" placeholder="选择端口分组" style="width: 100%" @change="applyPortGroup">
                <el-option v-for="g in portGroups" :key="g.value" :label="`${g.label}（${formatPortsSummary(g)}）`" :value="g.value" />
              </el-select>
              <span class="hint">按组预置端口，选中自动填充；选择「自定义」后可自由增删</span>
            </el-form-item>
            <el-form-item v-if="ruleDialog.form.portGroup === 'custom'" label="自定义端口">
              <el-select
                v-model="ruleDialog.form.ports"
                multiple
                filterable
                allow-create
                default-first-option
                clearable
                placeholder="选择或输入端口，输入后按回车添加"
                style="width: 100%"
              >
                <el-option v-for="p in commonPorts" :key="p.value" :label="p.label" :value="p.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="已选端口">
              <div class="port-tags">
                <el-tag v-if="ruleDialog.form.ports.length" size="small" type="info">
                  {{ ruleDialog.form.portGroup === 'custom' ? '自定义' : portGroupLabel(ruleDialog.form.portGroup) }}组 · {{ ruleDialog.form.ports.length }} 个端口
                </el-tag>
                <el-tag
                  v-for="(p, idx) in ruleDialog.form.ports.slice(0, 8)"
                  :key="p"
                  size="small"
                  closable
                  @close="removePort(p)"
                >
                  {{ p }}
                </el-tag>
                <el-tag v-if="ruleDialog.form.ports.length > 8" size="small" type="info">+{{ ruleDialog.form.ports.length - 8 }}</el-tag>
                <span v-if="!ruleDialog.form.ports.length" class="hint">（组内默认端口，保存时生效）</span>
              </div>
            </el-form-item>
            <el-form-item label="探活端口组">
              <el-select v-model="ruleDialog.form.probePortGroup" placeholder="选择探活端口分组" style="width: 100%" @change="applyProbeGroup">
                <el-option v-for="g in probeGroups" :key="g.value" :label="`${g.label}（${formatPortsSummary(g)}）`" :value="g.value" />
              </el-select>
              <span class="hint">大网段先扫探活端口定位存活主机，再详扫，可大幅提速</span>
            </el-form-item>
            <el-form-item v-if="ruleDialog.form.probePortGroup === 'custom'" label="自定义探活端口">
              <el-select
                v-model="ruleDialog.form.probePorts"
                multiple
                filterable
                allow-create
                default-first-option
                clearable
                placeholder="选择或输入探活端口"
                style="width: 100%"
              >
                <el-option v-for="p in probePortOptions" :key="p.value" :label="p.label" :value="p.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="调度间隔(分)">
              <el-input-number v-model="ruleDialog.form.intervalMinutes" :min="5" :max="10080" />
            </el-form-item>
            <el-form-item label="服务识别">
              <el-switch v-model="ruleDialog.form.serviceDetect" />
              <span class="hint">启用后扫描时执行 -sV 识别服务版本</span>
            </el-form-item>
            <el-form-item label="自动纳管">
              <el-switch v-model="ruleDialog.form.autoAdopt" />
              <span class="hint">启用后新发现主机自动入库（建议人工确认）</span>
            </el-form-item>
            <el-form-item label="启用">
              <el-switch v-model="ruleDialog.form.enabled" />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="ruleDialog.visible = false">取消</el-button>
            <el-button type="primary" :loading="savingRule" @click="saveRule">保存</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ============ 运行记录 ============ -->
      <el-tab-pane label="运行记录" name="runs">
        <div class="toolbar">
          <el-select v-model="runFilter.status" placeholder="状态" clearable style="width: 120px" @change="loadRuns">
            <el-option v-for="(item, key) in runStatusOptions" :key="key" :label="item.label" :value="key" />
          </el-select>
          <el-button @click="loadRuns">刷新</el-button>
        </div>
        <el-table :data="runs" v-loading="loadingRuns" border stripe @row-click="openRunDetail" style="cursor: pointer">
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column label="规则" min-width="140">
            <template #default="{ row }">{{ row.rule?.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="触发方式" width="100" align="center">
            <template #default="{ row }">{{ row.trigger === 'schedule' ? '定时' : '手动' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90" align="center">
            <template #default="{ row }">
              <el-tag :type="dictItem(runStatusMap, row.status).type" size="small">{{ dictItem(runStatusMap, row.status).label }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="newCount" label="新增" width="70" align="center" />
          <el-table-column prop="changedCount" label="变更" width="70" align="center" />
          <el-table-column prop="offlineCount" label="离线" width="70" align="center" />
          <el-table-column prop="totalHosts" label="发现主机" width="90" align="center" />
          <el-table-column label="开始时间" width="170">
            <template #default="{ row }">{{ formatTime(row.startedAt) }}</template>
          </el-table-column>
          <el-table-column prop="error" label="错误信息" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.error" class="error-text">{{ row.error }}</span>
              <span v-else>-</span>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination
          v-if="runTotal > runPageSize"
          layout="prev, pager, next"
          :total="runTotal"
          :page-size="runPageSize"
          :current-page="runPage"
          @current-change="(p) => { runPage = p; loadRuns() }"
          style="margin-top: 12px; justify-content: flex-end"
        />

        <!-- 运行详情 -->
        <el-drawer v-model="runDetail.visible" title="运行详情" size="70%">
          <template v-if="runDetail.run">
            <el-descriptions :column="3" border size="small" style="margin-bottom: 12px">
              <el-descriptions-item label="规则">{{ runDetail.run.rule?.name }}</el-descriptions-item>
              <el-descriptions-item label="状态">
                <el-tag :type="dictItem(runStatusMap, runDetail.run.status).type" size="small">{{ dictItem(runStatusMap, runDetail.run.status).label }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="开始时间">{{ formatTime(runDetail.run.startedAt) }}</el-descriptions-item>
              <el-descriptions-item label="新增">{{ runDetail.run.newCount }}</el-descriptions-item>
              <el-descriptions-item label="变更">{{ runDetail.run.changedCount }}</el-descriptions-item>
              <el-descriptions-item label="离线">{{ runDetail.run.offlineCount }}</el-descriptions-item>
            </el-descriptions>
            <el-alert v-if="runDetail.run.error" :title="runDetail.run.error" type="error" :closable="false" style="margin-bottom: 12px" />

            <div class="detail-toolbar">
              <span class="hint">勾选结果后操作：新增主机「纳管为资产」，变更/离线/恢复在线「应用到资产」</span>
            </div>
            <el-table :data="runDetail.run.hosts || []" border stripe size="small" @selection-change="(rows) => (selectedHosts = rows)">
              <el-table-column type="selection" width="45" :selectable="hostSelectable" />
              <el-table-column prop="ip" label="IP" width="130" />
              <el-table-column prop="hostname" label="主机名" min-width="130" show-overflow-tooltip />
              <el-table-column label="变化" width="90" align="center">
                <template #default="{ row }">
                  <el-tag :type="dictItem(changeTypeMap, row.changeType).type" size="small">{{ dictItem(changeTypeMap, row.changeType).label }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="os" label="操作系统" min-width="120" show-overflow-tooltip />
              <el-table-column prop="openPorts" label="开放端口" min-width="140" show-overflow-tooltip />
              <el-table-column label="匹配资产" width="130">
                <template #default="{ row }">{{ row.matchedAsset ? `${row.matchedAsset.assetNo} (${row.matchedAsset.hostname})` : '-' }}</template>
              </el-table-column>
              <el-table-column label="已处理" width="90" align="center">
                <template #default="{ row }">
                  <el-tag v-if="row.adopted || row.applied" type="success" size="small">已{{ row.adopted ? '纳管' : '应用' }}</el-tag>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column prop="diffSummary" label="差异摘要" min-width="220" show-overflow-tooltip />
            </el-table>
            <div class="detail-toolbar" style="margin-top: 12px">
              <el-button type="success" plain :disabled="!adoptableSelected.length" @click="adoptSelected">纳管新增主机 ({{ adoptableSelected.length }})</el-button>
              <el-button type="warning" plain :disabled="!applicableSelected.length" @click="applySelected">应用变更 ({{ applicableSelected.length }})</el-button>
            </div>
          </template>
        </el-drawer>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { discoveryApi } from '../api/discovery'
import {
  DISCOVERY_RUN_STATUS_MAP,
  DISCOVERY_CHANGE_TYPE_MAP,
  dictItem,
  dictOptions
} from '../constants/dictionaries'

const activeTab = ref('rules')

const rules = ref([])
const loadingRules = ref(false)
const runningRuleId = ref(0)
const testingRuleId = ref(0)

const ruleDialog = reactive({ visible: false, form: {} })
const savingRule = ref(false)

const emptyRuleForm = () => ({
  id: 0,
  name: '',
  targets: '',
  ports: [],
  probePorts: [],
  portGroup: 'common',
  probePortGroup: 'common',
  serviceDetect: false,
  intervalMinutes: 60,
  autoAdopt: false,
  enabled: true
})

// 端口分组（参考 Goby 的分类方式）：精简 / 常见 / 企业 / 全端口 / 自定义
const portGroups = [
  { value: 'lite', label: '精简', ports: ['22', '80', '443', '3389'] },
  { value: 'common', label: '常见', ports: ['22', '80', '135', '139', '443', '445', '3389', '8080', '8443'] },
  {
    value: 'enterprise',
    label: '企业',
    ports: [
      '21', '22', '23', '25', '53', '80', '110', '135', '139', '143', '443', '445',
      '993', '995', '1433', '3306', '3389', '5432', '6379', '8080', '8443', '9200', '11211', '27017'
    ]
  },
  { value: 'all', label: '全端口', ports: ['1-65535'] },
  { value: 'custom', label: '自定义', ports: [] }
]

// 探活端口分组
const probeGroups = [
  { value: 'lite', label: '精简', ports: ['22', '80', '443', '3389'] },
  { value: 'common', label: '常见', ports: ['22', '80', '135', '139', '443', '445', '3389', '8080'] },
  { value: 'custom', label: '自定义', ports: [] }
]

// 常见端口选项（自定义时供勾选，可自由输入）
const commonPorts = [
  { value: '22', label: '22 SSH' },
  { value: '21', label: '21 FTP' },
  { value: '23', label: '23 Telnet' },
  { value: '25', label: '25 SMTP' },
  { value: '53', label: '53 DNS' },
  { value: '80', label: '80 HTTP' },
  { value: '110', label: '110 POP3' },
  { value: '135', label: '135 MSRPC' },
  { value: '139', label: '139 NetBIOS' },
  { value: '143', label: '143 IMAP' },
  { value: '443', label: '443 HTTPS' },
  { value: '445', label: '445 SMB' },
  { value: '993', label: '993 IMAPS' },
  { value: '995', label: '995 POP3S' },
  { value: '1433', label: '1433 MSSQL' },
  { value: '3306', label: '3306 MySQL' },
  { value: '3389', label: '3389 RDP' },
  { value: '5432', label: '5432 PostgreSQL' },
  { value: '6379', label: '6379 Redis' },
  { value: '8080', label: '8080 HTTP' },
  { value: '8443', label: '8443 HTTPS' },
  { value: '9200', label: '9200 Elasticsearch' },
  { value: '11211', label: '11211 Memcached' },
  { value: '27017', label: '27017 MongoDB' }
]

const probePortOptions = [
  { value: '22', label: '22 SSH' },
  { value: '80', label: '80 HTTP' },
  { value: '135', label: '135 MSRPC' },
  { value: '139', label: '139 NetBIOS' },
  { value: '443', label: '443 HTTPS' },
  { value: '445', label: '445 SMB' },
  { value: '3389', label: '3389 RDP' },
  { value: '8080', label: '8080 HTTP' }
]

// 逗号分隔字符串 ↔ 多选数组
function portsToArray(value) {
  if (!value) return []
  return String(value)
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

function portsToString(value) {
  if (!Array.isArray(value) || value.length === 0) return ''
  return value.join(',')
}

// 分组摘要：全端口显示范围，其余显示端口数量（避免下拉选项过长）
function formatPortsSummary(group) {
  if (group.ports.length === 1 && group.ports[0] === '1-65535') return '1-65535'
  return `${group.ports.length} 个端口`
}

function portGroupLabel(value) {
  return portGroups.find((g) => g.value === value)?.label || value
}

// 根据端口列表匹配所属分组（精确匹配返回组，否则 custom）
function matchGroup(ports, groups) {
  if (!ports.length) return 'custom'
  for (const g of groups) {
    if (g.ports.length && g.ports.length === ports.length && [...g.ports].sort().join(',') === [...ports].sort().join(',')) {
      return g.value
    }
  }
  return 'custom'
}

// 选择端口组时自动填充
function applyPortGroup(value) {
  const group = portGroups.find((g) => g.value === value)
  if (group) {
    ruleDialog.form.ports = [...group.ports]
  }
}

function applyProbeGroup(value) {
  const group = probeGroups.find((g) => g.value === value)
  if (group) {
    ruleDialog.form.probePorts = [...group.ports]
  }
}

// 非自定义组模式下移除单个端口（转为自定义）
function removePort(p) {
  ruleDialog.form.ports = ruleDialog.form.ports.filter((item) => item !== p)
  ruleDialog.form.portGroup = 'custom'
}

const runStatusMap = DISCOVERY_RUN_STATUS_MAP
const changeTypeMap = DISCOVERY_CHANGE_TYPE_MAP
const runStatusOptions = dictOptions(DISCOVERY_RUN_STATUS_MAP)

const runs = ref([])
const loadingRuns = ref(false)
const runFilter = reactive({ status: '' })
const runPage = ref(1)
const runPageSize = 20
const runTotal = ref(0)

const runDetail = reactive({ visible: false, run: null })
const selectedHosts = ref([])

const adoptableSelected = computed(() => selectedHosts.value.filter((h) => h.changeType === 'new'))
const applicableSelected = computed(() =>
  selectedHosts.value.filter((h) => ['changed', 'offline', 'online'].includes(h.changeType))
)

function hostSelectable(row) {
  if (row.adopted || row.applied) return false
  return ['new', 'changed', 'offline', 'online'].includes(row.changeType)
}

function formatTime(value) {
  if (!value) return '-'
  const d = new Date(value)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function loadRules() {
  loadingRules.value = true
  try {
    rules.value = await discoveryApi.listRules()
  } catch (e) {
    ElMessage.error(e.message || '加载规则失败')
  } finally {
    loadingRules.value = false
  }
}

function openRuleDialog(row) {
  if (row) {
    const ports = portsToArray(row.ports)
    const probePorts = portsToArray(row.probePorts)
    ruleDialog.form = {
      ...row,
      ports,
      probePorts,
      portGroup: matchGroup(ports, portGroups),
      probePortGroup: matchGroup(probePorts, probeGroups)
    }
  } else {
    ruleDialog.form = emptyRuleForm()
    applyPortGroup(ruleDialog.form.portGroup)
    applyProbeGroup(ruleDialog.form.probePortGroup)
  }
  ruleDialog.visible = true
}

async function saveRule() {
  const form = ruleDialog.form
  if (!form.name || !form.targets) {
    ElMessage.warning('请填写规则名称与扫描目标')
    return
  }
  savingRule.value = true
  try {
    // 多选数组 → 逗号分隔字符串提交（剔除前端组字段）
    const payload = {
      ...form,
      ports: portsToString(form.ports),
      probePorts: portsToString(form.probePorts),
      portGroup: undefined,
      probePortGroup: undefined
    }
    if (form.id) {
      await discoveryApi.updateRule(form.id, payload)
    } else {
      await discoveryApi.createRule(payload)
    }
    ElMessage.success('保存成功')
    ruleDialog.visible = false
    await loadRules()
  } catch (e) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    savingRule.value = false
  }
}

async function removeRule(row) {
  try {
    await ElMessageBox.confirm(`确定删除规则「${row.name}」？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await discoveryApi.removeRule(row.id)
    ElMessage.success('已删除')
    await loadRules()
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

async function runRule(row) {
  runningRuleId.value = row.id
  try {
    await discoveryApi.runRule(row.id)
    ElMessage.success('发现任务已启动，可在「运行记录」中查看')
  } catch (e) {
    ElMessage.error(e.message || '启动失败')
  } finally {
    runningRuleId.value = 0
  }
}

async function testRule(row) {
  testingRuleId.value = row.id
  try {
    const res = await discoveryApi.testRule(row.id)
    if (res.ok) {
      ElMessage.success(`试跑成功：共 ${res.total} 台，在线 ${res.up} 台（nmap: ${res.nmapBin}）`)
    } else {
      ElMessageBox.alert(res.error + '，' + (res.hint || ''), 'nmap 不可用', { type: 'error' })
    }
  } catch (e) {
    ElMessage.error(e.message || '试跑失败')
  } finally {
    testingRuleId.value = 0
  }
}

async function loadRuns() {
  loadingRuns.value = true
  try {
    const params = { page: runPage.value, pageSize: runPageSize }
    if (runFilter.status) params.status = runFilter.status
    const res = await discoveryApi.listRuns(params)
    runs.value = res.items || []
    runTotal.value = res.total || 0
  } catch (e) {
    ElMessage.error(e.message || '加载运行记录失败')
  } finally {
    loadingRuns.value = false
  }
}

async function openRunDetail(row) {
  try {
    runDetail.run = await discoveryApi.getRun(row.id)
    runDetail.visible = true
    selectedHosts.value = []
  } catch (e) {
    ElMessage.error(e.message || '加载详情失败')
  }
}

async function adoptSelected() {
  const ids = adoptableSelected.value.map((h) => h.id)
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(`确认将 ${ids.length} 台新发现主机纳管为资产？`, '纳管确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    const res = await discoveryApi.adoptHosts(runDetail.run.id, ids)
    ElMessage.success(`已纳管 ${res.adopted} 台主机`)
    await openRunDetail(runDetail.run)
    await loadRuns()
  } catch (e) {
    ElMessage.error(e.message || '纳管失败')
  }
}

async function applySelected() {
  const ids = applicableSelected.value.map((h) => h.id)
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(`确认将 ${ids.length} 条变更应用到资产台账？`, '应用确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    const res = await discoveryApi.applyHosts(runDetail.run.id, ids)
    ElMessage.success(`已应用 ${res.applied} 条变更`)
    await openRunDetail(runDetail.run)
    await loadRuns()
  } catch (e) {
    ElMessage.error(e.message || '应用失败')
  }
}

onMounted(() => {
  loadRules()
  loadRuns()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.hint {
  color: #6b7280;
  font-size: 12px;
}
.error-text {
  color: #dc2626;
  font-size: 12px;
}
.detail-toolbar {
  margin-bottom: 8px;
}

.port-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  min-height: 24px;
}
</style>
