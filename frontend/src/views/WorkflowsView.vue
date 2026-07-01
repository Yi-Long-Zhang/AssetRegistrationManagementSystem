<template>
  <section class="page">
    <PageHeader title="流程配置" />
    <div class="panel">
      <el-tabs v-model="activeType" @tab-change="loadWorkflow">
        <el-tab-pane v-for="option in ticketTypes" :key="option.value" :label="option.label" :name="option.value" />
      </el-tabs>
      <el-form :model="form" label-width="90px">
        <el-form-item label="流程名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <div class="toolbar">
        <h3>审批节点</h3>
        <el-button type="primary" @click="addNode">新增节点</el-button>
      </div>
      <div class="nodes">
        <div v-for="(node, index) in form.nodes" :key="node.localId" class="node-row">
          <div class="node-order">{{ index + 1 }}</div>
          <el-input v-model="node.name" placeholder="节点名称，如 IT运维主管审核" />
          <el-select v-model="node.approverIds" multiple filterable placeholder="选择审批人">
            <el-option v-for="user in approverUsers" :key="user.id" :label="`${user.name} (${user.username})`" :value="user.id" />
          </el-select>
          <el-button :disabled="index === 0" @click="moveNode(index, -1)">上移</el-button>
          <el-button :disabled="index === form.nodes.length - 1" @click="moveNode(index, 1)">下移</el-button>
          <el-button type="danger" link @click="removeNode(index)">删除</el-button>
        </div>
      </div>
      <div class="form-actions">
        <el-button @click="loadWorkflow">重置</el-button>
        <el-button type="primary" @click="saveWorkflow">保存流程</el-button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { usersApi, workflowsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'
import { TICKET_TYPE_MAP, dictOptions } from '../constants/dictionaries'

const ticketTypes = dictOptions(TICKET_TYPE_MAP)
const activeType = ref(ticketTypes[0]?.value || 'asset_register')
const users = ref([])
const form = reactive({ name: '', enabled: true, nodes: [] })
const approverUsers = computed(() => users.value.filter((user) => user.status === 'active'))

function blankNode(name = '') {
  return { localId: `${Date.now()}-${Math.random()}`, name, approverIds: [] }
}

async function loadUsers() {
  const data = await usersApi.list()
  users.value = data.items || []
}

async function loadWorkflow() {
  try {
    const data = await workflowsApi.detail(activeType.value)
    form.name = data.name || `${TICKET_TYPE_MAP[activeType.value]?.label || activeType.value}流程`
    form.enabled = data.enabled !== false
    form.nodes = (data.nodes || []).map((node) => ({
      localId: `${node.id}-${Math.random()}`,
      name: node.name,
      approverIds: (node.approvers || []).map((item) => item.userId)
    }))
  } catch {
    form.name = `${TICKET_TYPE_MAP[activeType.value]?.label || activeType.value}流程`
    form.enabled = true
    form.nodes = [blankNode('IT运维主管审核'), blankNode('信息技术部经理审批')]
  }
}

function addNode() {
  form.nodes.push(blankNode())
}

function removeNode(index) {
  form.nodes.splice(index, 1)
}

function moveNode(index, offset) {
  const target = index + offset
  const current = form.nodes.splice(index, 1)[0]
  form.nodes.splice(target, 0, current)
}

async function saveWorkflow() {
  if (!form.nodes.length) {
    ElMessage.warning('至少需要一个审批节点')
    return
  }
  for (const node of form.nodes) {
    if (!node.name.trim() || !node.approverIds.length) {
      ElMessage.warning('节点名称和审批人不能为空')
      return
    }
  }
  await workflowsApi.save(activeType.value, {
    name: form.name,
    enabled: form.enabled,
    nodes: form.nodes.map((node) => ({ name: node.name, approverIds: node.approverIds }))
  })
  ElMessage.success('流程配置已保存')
  await loadWorkflow()
}

onMounted(async () => {
  await loadUsers()
  await loadWorkflow()
})
</script>

<style scoped>
.nodes {
  display: grid;
  gap: 12px;
}

.node-row {
  align-items: center;
  display: grid;
  gap: 8px;
  grid-template-columns: 42px minmax(180px, 1fr) minmax(240px, 1.4fr) auto auto auto;
}

.node-order {
  align-items: center;
  background: #eef2ff;
  border-radius: 6px;
  color: #1d4ed8;
  display: flex;
  font-weight: 700;
  height: 32px;
  justify-content: center;
}

.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
