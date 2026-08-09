<template>
  <AppLayout>
    <div class="dashboard">
      <!-- 概览卡片 -->
      <div class="stat-grid">
        <div class="stat-card">
          <div class="stat-icon icon-total">📦</div>
          <div class="stat-body">
            <div class="stat-num">{{ stats.total || 0 }}</div>
            <div class="stat-label">资产总数</div>
          </div>
        </div>
      <div class="stat-card">
        <div class="stat-icon icon-online">🟢</div>
        <div class="stat-body">
          <div class="stat-num">{{ stats.byOnlineStatus?.online || 0 }}</div>
          <div class="stat-label">在线</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon icon-offline">🔴</div>
        <div class="stat-body">
          <div class="stat-num">{{ stats.byOnlineStatus?.offline || 0 }}</div>
          <div class="stat-label">离线</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon icon-unknown">⚪</div>
        <div class="stat-body">
          <div class="stat-num">{{ stats.byOnlineStatus?.unknown || 0 }}</div>
          <div class="stat-label">未知</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon icon-port">🌐</div>
        <div class="stat-body">
          <div class="stat-num">{{ stats.openPortAssetCount || 0 }}</div>
          <div class="stat-label">开放端口资产</div>
        </div>
      </div>
    </div>

    <div class="dash-row">
      <!-- 资产类型分布 -->
      <div class="panel">
        <div class="panel-title">资产类型分布</div>
        <div v-for="item in stats.byAssetType || []" :key="item.label" class="type-row">
          <span class="type-name">{{ typeLabel(item.label) }}</span>
          <div class="type-bar-wrap">
            <div class="type-bar" :style="{ width: ratio(item.count, typeMax) + '%' }"></div>
          </div>
          <span class="type-count">{{ item.count }}</span>
        </div>
        <el-empty v-if="!stats.byAssetType?.length" description="暂无数据" :image-size="60" />
      </div>

      <!-- 发现趋势 -->
      <div class="panel">
        <div class="panel-title">发现趋势（近 14 天）</div>
        <div class="mini-chart">
          <div v-for="item in trend" :key="item.date" class="mini-col" :title="`${item.date} 新增${item.new} 变更${item.changed} 离线${item.offline}`">
            <div class="mini-bars">
              <div class="mini-bar mb-new" :style="{ height: miniH(item.new, trendMax) + '%' }"></div>
              <div class="mini-bar mb-changed" :style="{ height: miniH(item.changed, trendMax) + '%' }"></div>
            </div>
          </div>
        </div>
        <div class="mini-legend">
          <span><i class="dot dot-new"></i>新增</span>
          <span><i class="dot dot-changed"></i>变更</span>
        </div>
      </div>
    </div>

    <!-- 最近运行 -->
    <div class="panel">
      <div class="panel-title">最近发现运行</div>
      <el-table :data="recentRuns" size="small" border stripe v-loading="loadingRuns">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="规则" min-width="140">
          <template #default="{ row }">{{ row.rule?.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="trigger" label="触发方式" width="100" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="dictItem(runStatusMap, row.status).type" size="small">{{ dictItem(runStatusMap, row.status).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="newCount" label="新增" width="70" align="center" />
        <el-table-column prop="changedCount" label="变更" width="70" align="center" />
        <el-table-column prop="offlineCount" label="离线" width="70" align="center" />
        <el-table-column label="时间" min-width="150">
          <template #default="{ row }">{{ row.startedAt ? formatTime(row.startedAt) : '-' }}</template>
        </el-table-column>
      </el-table>
    </div>
    </div>
  </AppLayout>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import AppLayout from '../layouts/AppLayout.vue'
import { assetsApi } from '../api/assets'
import { discoveryApi } from '../api/discovery'
import { ASSET_TYPE_MAP, DISCOVERY_RUN_STATUS_MAP, dictItem } from '../constants/dictionaries'

const stats = ref({})
const trend = ref([])
const recentRuns = ref([])
const loadingRuns = ref(false)

const runStatusMap = DISCOVERY_RUN_STATUS_MAP

const typeMax = computed(() => {
  let max = 0
  ;(stats.value.byAssetType || []).forEach((item) => {
    max = Math.max(max, item.count || 0)
  })
  return max
})

const trendMax = computed(() => {
  let max = 0
  trend.value.forEach((item) => {
    max = Math.max(max, item.new || 0, item.changed || 0)
  })
  return max
})

function ratio(count, max) {
  if (!max || !count) return 2
  return Math.max(2, Math.round((count / max) * 100))
}

function miniH(value, max) {
  if (!max || !value) return 2
  return Math.max(2, Math.round((value / max) * 100))
}

function typeLabel(type) {
  return dictItem(ASSET_TYPE_MAP, type).label
}

function formatTime(value) {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}

async function load() {
  try {
    stats.value = await assetsApi.stats()
  } catch (e) {
    console.error('load stats:', e)
  }
  try {
    trend.value = await discoveryApi.getTrend(14)
  } catch (e) {
    console.error('load trend:', e)
  }
  loadingRuns.value = true
  try {
    const res = await discoveryApi.listRuns({ page: 1, pageSize: 5 })
    recentRuns.value = Array.isArray(res) ? res : res.list || []
  } catch (e) {
    console.error('load runs:', e)
  } finally {
    loadingRuns.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
}
.stat-card {
  display: flex;
  align-items: center;
  gap: 12px;
  background: linear-gradient(135deg, #ffffff, #f5f7ff);
  border: 1px solid #e5e9f5;
  border-radius: 14px;
  padding: 16px;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.stat-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 20px rgba(79, 70, 229, 0.12);
}
.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
}
.icon-total {
  background: #eef2ff;
}
.icon-online {
  background: #ecfdf5;
}
.icon-offline {
  background: #fef2f2;
}
.icon-unknown {
  background: #f3f4f6;
}
.icon-port {
  background: #eff6ff;
}
.stat-num {
  font-size: 26px;
  font-weight: 700;
  color: #1f2937;
}
.stat-label {
  font-size: 12px;
  color: #6b7280;
}
.dash-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
@media (max-width: 900px) {
  .dash-row {
    grid-template-columns: 1fr;
  }
}
.panel {
  background: #fff;
  border: 1px solid #e5e9f5;
  border-radius: 14px;
  padding: 16px;
}
.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 14px;
}
.type-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.type-name {
  width: 90px;
  font-size: 13px;
  color: #4b5563;
  text-align: right;
  white-space: nowrap;
}
.type-bar-wrap {
  flex: 1;
  height: 18px;
  background: #f3f4f6;
  border-radius: 5px;
  overflow: hidden;
}
.type-bar {
  height: 100%;
  background: linear-gradient(90deg, #6366f1, #8b5cf6);
  border-radius: 5px;
  transition: width 0.4s ease;
}
.type-count {
  width: 40px;
  font-size: 13px;
  color: #6b7280;
}
.mini-chart {
  display: flex;
  gap: 4px;
  align-items: flex-end;
  height: 140px;
  border-bottom: 1px solid #e5e9f5;
}
.mini-col {
  flex: 1;
  display: flex;
  align-items: flex-end;
}
.mini-bars {
  display: flex;
  gap: 2px;
  width: 100%;
  height: 100%;
}
.mini-bar {
  flex: 1;
  border-radius: 2px 2px 0 0;
  min-height: 2px;
}
.mb-new {
  background: linear-gradient(180deg, #34d399, #10b981);
}
.mb-changed {
  background: linear-gradient(180deg, #fbbf24, #f59e0b);
}
.mini-legend {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  font-size: 12px;
  color: #6b7280;
}
.mini-legend .dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 3px;
  margin-right: 4px;
}
.dot-new {
  background: #10b981;
}
.dot-changed {
  background: #f59e0b;
}
</style>
