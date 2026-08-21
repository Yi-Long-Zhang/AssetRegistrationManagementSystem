<template>
  <section class="page">
    <PageHeader title="机柜视图" />
    <div class="panel">
      <div class="toolbar">
        <el-select v-model="roomId" placeholder="选择机房" style="width: 220px" @change="loadRacks">
          <el-option v-for="room in rooms" :key="room.id" :label="room.name" :value="room.id" />
        </el-select>
        <span class="muted">{{ currentRoom?.location || '' }}</span>
      </div>
      <div class="rack-grid">
        <el-card v-for="rack in racks" :key="rack.id" class="rack-card" :class="{ active: activeRack?.id === rack.id }" shadow="hover" @click="selectRack(rack)">
          <div class="rack-name">{{ rack.name }}</div>
          <div class="rack-units">{{ rack.units }}U · {{ rackAssetsCount(rack.name) }} 资产</div>
        </el-card>
      </div>
      <el-empty v-if="roomId && !racks.length" description="该机房暂无机柜，请先在「机房机柜」页添加" />

      <div v-if="activeRack" class="rack-view">
        <div class="rack-view-head">
          <h3>{{ activeRack.name }}（{{ activeRack.units }}U）</h3>
          <el-button size="small" @click="openAssets">查看机柜内资产列表</el-button>
        </div>
        <div ref="chartRef" class="chart" />
      </div>
    </div>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { CustomChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { rackApi, assetsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

echarts.use([CustomChart, GridComponent, TooltipComponent, CanvasRenderer])

const router = useRouter()
const rooms = ref([])
const racks = ref([])
const roomId = ref(null)
const currentRoom = ref(null)
const activeRack = ref(null)
const chartRef = ref(null)
let chart = null

// rackName -> 资产列表
const rackAssets = ref({})

async function loadRooms() {
  const data = await rackApi.listRooms()
  rooms.value = data.items || []
  if (rooms.value.length) {
    roomId.value = rooms.value[0].id
    await loadRacks()
  }
}

async function loadRacks() {
  currentRoom.value = rooms.value.find((r) => r.id === roomId.value) || null
  const data = await rackApi.listRacks({ roomId: roomId.value })
  racks.value = data.items || []
  rackAssets.value = {}
  await Promise.all(racks.value.map((rack) => loadRackAssets(rack.name)))
  activeRack.value = null
}

async function loadRackAssets(rackName) {
  try {
    const data = await assetsApi.list({ rack: rackName, page: 1, pageSize: 500 })
    rackAssets.value[rackName] = data.items || []
  } catch {
    rackAssets.value[rackName] = []
  }
}

function rackAssetsCount(rackName) {
  return (rackAssets.value[rackName] || []).length
}

function selectRack(rack) {
  activeRack.value = rack
  renderChart(rack)
}

function parsePosition(value) {
  if (!value) return null
  const m = String(value).trim().match(/^(\d+)\s*(?:-\s*(\d+))?$/)
  if (!m) return null
  const start = parseInt(m[1], 10)
  const end = m[2] ? parseInt(m[2], 10) : start
  if (start < 1 || end < start) return null
  return { start, end }
}

function renderChart(rack) {
  const el = chartRef.value
  if (!el) return
  if (!chart) chart = echarts.init(el)
  const assets = (rackAssets.value[rack.name] || []).map((a) => ({ ...a, pos: parsePosition(a.rackPosition) })).filter((a) => a.pos)
  const units = rack.units || 42

  // 数据项：每个 U 位一个格子
  const data = []
  for (let u = 1; u <= units; u++) {
    data.push({ value: u, kind: 'cell' })
  }
  for (const a of assets) {
    data.push({ value: a.pos.start, kind: 'asset', asset: a, start: a.pos.start, end: a.pos.end })
  }

  chart.setOption({
    animation: true,
    animationDuration: 400,
    animationEasing: 'cubicOut',
    animationDurationUpdate: 300,
    animationEasingUpdate: 'cubicOut',
    tooltip: {
      trigger: 'item',
      formatter: (params) => {
        if (params.data && params.data.kind === 'asset') {
          const a = params.data.asset
          return `<b>${a.hostname || a.assetNo}</b><br/>IP：${a.ip}<br/>类型：${a.assetType || '-'}<br/>U 位：${a.start === a.end ? a.start : a.start + '-' + a.end}`
        }
        return `U${params.value}`
      }
    },
    grid: { left: 40, right: 20, top: 10, bottom: 20 },
    xAxis: { type: 'value', min: 0, max: 1, show: false },
    yAxis: {
      type: 'category',
      data: Array.from({ length: units }, (_, i) => units - i), // 顶部 1U 起（倒序）
      inverse: true,
      axisLabel: { fontSize: 11 },
      axisLine: { show: false },
      axisTick: { show: false }
    },
    series: [{
      type: 'custom',
      renderItem: (params, api) => {
        const catIndex = api.value(0)
        const point = api.coord([0, catIndex])
        const size = api.size([1, 0.86])
        const y = point[1] - size[1] / 2
        const x = point[0]
        const dataItem = params.data
        if (dataItem.kind === 'asset') {
          const a = dataItem.asset
          const span = dataItem.end - dataItem.start + 1
          const h = size[1] * span
          const rect = { type: 'rect', shape: { x: x, y: point[1] - h / 2, width: size[0], height: h }, style: { fill: '#409EFF', opacity: 0.85, stroke: '#337ecc' } }
          const text = {
            type: 'text',
            style: {
              text: a.hostname || a.assetNo,
              x: x + size[0] / 2,
              y: point[1],
              textAlign: 'center',
              textVerticalAlign: 'middle',
              fill: '#fff',
              fontSize: 11
            }
          }
          return { type: 'group', children: [rect, text] }
        }
        return {
          type: 'rect',
          shape: { x: x, y: y, width: size[0], height: size[1] },
          style: { fill: '#f5f7fa', stroke: '#dcdfe6', lineWidth: 1 }
        }
      },
      encode: { x: [0], y: [0] },
      data
    }]
  })
  chart.off('click')
  chart.on('click', (params) => {
    if (params.data && params.data.kind === 'asset' && params.data.asset) {
      router.push({ path: '/assets', query: { assetId: params.data.asset.id } })
    }
  })
}

function openAssets() {
  router.push({ path: '/assets', query: { rack: activeRack.value?.name } })
}

onMounted(async () => {
  await loadRooms()
})

onBeforeUnmount(() => {
  if (chart) {
    chart.dispose()
    chart = null
  }
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.muted {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.rack-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
}

.rack-card {
  width: 150px;
  cursor: pointer;
}

.rack-card.active {
  border-color: var(--el-color-primary);
}

.rack-name {
  font-weight: 600;
}

.rack-units {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.rack-view-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.chart {
  width: 100%;
  height: 560px;
}
</style>
