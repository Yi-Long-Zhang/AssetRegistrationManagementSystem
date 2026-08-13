<template>
  <section class="page">
    <PageHeader title="机房机柜" />
    <div class="panel">
      <div class="layout">
        <div class="rooms-pane">
          <div class="toolbar">
            <h3>机房</h3>
            <el-button type="primary" size="small" @click="openRoom()">新增机房</el-button>
          </div>
          <el-table :data="rooms" v-loading="loading" highlight-current-row @current-change="selectRoom" size="small">
            <el-table-column prop="name" label="名称" min-width="120" />
            <el-table-column prop="location" label="位置" min-width="120" />
            <el-table-column label="操作" width="90">
              <template #default="{ row }">
                <el-button link type="primary" @click.stop="openRoom(row)">编辑</el-button>
                <el-button link type="danger" @click.stop="removeRoom(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="racks-pane">
          <div class="toolbar">
            <h3>机柜{{ currentRoom ? '（' + currentRoom.name + '）' : '' }}</h3>
            <el-button type="primary" size="small" :disabled="!currentRoom" @click="openRack()">新增机柜</el-button>
          </div>
          <el-table :data="racks" v-loading="loading" size="small">
            <el-table-column prop="name" label="名称" min-width="120" />
            <el-table-column prop="units" label="U 位数" width="90" />
            <el-table-column prop="remark" label="备注" min-width="140" show-overflow-tooltip />
            <el-table-column label="操作" width="120">
              <template #default="{ row }">
                <el-button link type="primary" @click="openRack(row)">编辑</el-button>
                <el-button link type="danger" @click="removeRack(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </div>

    <el-dialog v-model="roomDialog" :title="roomForm.id ? '编辑机房' : '新增机房'" width="440px">
      <el-form :model="roomForm" label-width="70px">
        <el-form-item label="名称" required><el-input v-model="roomForm.name" placeholder="如：A栋-3F机房" /></el-form-item>
        <el-form-item label="位置"><el-input v-model="roomForm.location" placeholder="地址/楼层" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="roomForm.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roomDialog = false">取消</el-button>
        <el-button type="primary" @click="saveRoom">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rackDialog" :title="rackForm.id ? '编辑机柜' : '新增机柜'" width="440px">
      <el-form :model="rackForm" label-width="70px">
        <el-form-item label="机房" required>
          <el-select v-model="rackForm.roomId" style="width: 100%">
            <el-option v-for="room in rooms" :key="room.id" :label="room.name" :value="room.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称" required><el-input v-model="rackForm.name" placeholder="如：A-01" /></el-form-item>
        <el-form-item label="U 位数"><el-input-number v-model="rackForm.units" :min="1" :max="60" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="rackForm.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rackDialog = false">取消</el-button>
        <el-button type="primary" @click="saveRack">保存</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { rackApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const rooms = ref([])
const racks = ref([])
const currentRoom = ref(null)
const loading = ref(false)
const roomDialog = ref(false)
const rackDialog = ref(false)
const roomForm = reactive({ id: null, name: '', location: '', remark: '' })
const rackForm = reactive({ id: null, roomId: null, name: '', units: 42, remark: '' })

async function loadRooms() {
  loading.value = true
  try {
    const data = await rackApi.listRooms()
    rooms.value = data.items || []
    if (!currentRoom.value && rooms.value.length) {
      selectRoom(rooms.value[0])
    }
  } finally {
    loading.value = false
  }
}

function selectRoom(row) {
  currentRoom.value = row
  loadRacks()
}

async function loadRacks() {
  if (!currentRoom.value) {
    racks.value = []
    return
  }
  const data = await rackApi.listRacks({ roomId: currentRoom.value.id })
  racks.value = data.items || []
}

function openRoom(row) {
  Object.assign(roomForm, row ? { id: row.id, name: row.name, location: row.location, remark: row.remark } : { id: null, name: '', location: '', remark: '' })
  roomDialog.value = true
}

async function saveRoom() {
  if (!roomForm.name.trim()) return ElMessage.warning('请输入机房名称')
  if (roomForm.id) {
    await rackApi.updateRoom(roomForm.id, roomForm)
  } else {
    await rackApi.createRoom(roomForm)
  }
  ElMessage.success('已保存')
  roomDialog.value = false
  await loadRooms()
}

async function removeRoom(row) {
  await ElMessageBox.confirm(`删除机房「${row.name}」将连带删除其机柜，确认？`, '删除确认', { type: 'warning' })
  await rackApi.removeRoom(row.id)
  ElMessage.success('已删除')
  if (currentRoom.value?.id === row.id) {
    currentRoom.value = null
    racks.value = []
  }
  await loadRooms()
}

function openRack(row) {
  Object.assign(rackForm, row ? { id: row.id, roomId: row.roomId, name: row.name, units: row.units, remark: row.remark } : { id: null, roomId: currentRoom.value?.id ?? null, name: '', units: 42, remark: '' })
  rackDialog.value = true
}

async function saveRack() {
  if (!rackForm.name.trim() || !rackForm.roomId) return ElMessage.warning('请填写机柜名称与机房')
  if (rackForm.id) {
    await rackApi.updateRack(rackForm.id, rackForm)
  } else {
    await rackApi.createRack(rackForm)
  }
  ElMessage.success('已保存')
  rackDialog.value = false
  await loadRooms()
  await loadRacks()
}

async function removeRack(row) {
  await ElMessageBox.confirm(`删除机柜「${row.name}」？`, '删除确认', { type: 'warning' })
  await rackApi.removeRack(row.id)
  ElMessage.success('已删除')
  await loadRacks()
}

onMounted(loadRooms)
</script>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.toolbar h3 {
  margin: 0;
}
</style>
