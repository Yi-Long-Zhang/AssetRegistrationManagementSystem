<template>
  <section class="page">
    <PageHeader title="工单报表" />
    <div class="panel">
      <div class="toolbar">
        <h3>工单统计报表</h3>
        <el-button type="primary" @click="exportCsv">导出 CSV</el-button>
      </div>

      <div class="cards">
        <div class="stat-card">
          <div class="stat-value">{{ stats.total ?? 0 }}</div>
          <div class="stat-label">工单总数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ slaRateText }}</div>
          <div class="stat-label">SLA 达标率（已关闭且带 SLA）</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.slaSummary?.total ?? 0 }}</div>
          <div class="stat-label">SLA 统计工单数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.slaSummary?.overdue ?? 0 }}</div>
          <div class="stat-label">SLA 超时工单</div>
        </div>
      </div>

      <el-row :gutter="16" class="chart-row">
        <el-col :span="12">
          <div class="chart-card">
            <h4>状态分布</h4>
            <div v-for="item in stats.byStatus" :key="item.label" class="bar-row">
              <span class="bar-label">{{ statusLabel(item.label) }}</span>
              <div class="bar-track"><div class="bar-fill" :style="{ width: barWidth(item.count) }" /></div>
              <span class="bar-count">{{ item.count }}</span>
            </div>
            <el-empty v-if="!stats.byStatus?.length" description="暂无数据" :image-size="60" />
          </div>
        </el-col>
        <el-col :span="12">
          <div class="chart-card">
            <h4>类型分布</h4>
            <div v-for="item in stats.byType" :key="item.label" class="bar-row">
              <span class="bar-label">{{ typeLabel(item.label) }}</span>
              <div class="bar-track"><div class="bar-fill type" :style="{ width: barWidth(item.count) }" /></div>
              <span class="bar-count">{{ item.count }}</span>
            </div>
            <el-empty v-if="!stats.byType?.length" description="暂无数据" :image-size="60" />
          </div>
        </el-col>
      </el-row>

      <el-row :gutter="16" class="chart-row">
        <el-col :span="12">
          <div class="chart-card">
            <h4>优先级分布</h4>
            <div v-for="item in stats.byPriority" :key="item.label" class="bar-row">
              <span class="bar-label">{{ priorityLabel(item.label) }}</span>
              <div class="bar-track"><div class="bar-fill priority" :style="{ width: barWidth(item.count) }" /></div>
              <span class="bar-count">{{ item.count }}</span>
            </div>
            <el-empty v-if="!stats.byPriority?.length" description="暂无数据" :image-size="60" />
          </div>
        </el-col>
        <el-col :span="12">
          <div class="chart-card">
            <h4>月度趋势（近 12 个月）</h4>
            <div class="trend">
              <div v-for="item in stats.monthlyTrend" :key="item.label" class="trend-col">
                <div class="trend-value">{{ item.count }}</div>
                <div class="trend-bar" :style="{ height: trendHeight(item.count) }" />
                <div class="trend-label">{{ item.label.slice(2) }}</div>
              </div>
            </div>
          </div>
        </el-col>
      </el-row>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive } from 'vue'
import { ticketsApi } from '../api'
import PageHeader from '../components/common/PageHeader.vue'

const stats = reactive({ total: 0, byType: [], byStatus: [], byPriority: [], monthlyTrend: [], slaSummary: { total: 0, met: 0, overdue: 0, rate: 0, applicable: false } })

const slaRateText = computed(() => (stats.slaSummary?.applicable ? `${stats.slaSummary.rate?.toFixed(1) ?? '0.0'}%` : '未启用/无数据'))

const STATUS_LABELS = {
  draft: '草稿', pending_approval: '审批中', approved: '已审批', rejected: '已驳回',
  in_progress: '执行中', pending_acceptance: '待验收', closed: '已关闭', cancelled: '已取消'
}
const TYPE_LABELS = {
  asset_register: '资产登记', asset_change: '资产变更', asset_retire: '资产下线/报废',
  maintenance: '权限/维护申请', inspection: '定期巡检'
}
const PRIORITY_LABELS = { low: '低', normal: '普通', high: '高', urgent: '紧急' }

function statusLabel(key) { return STATUS_LABELS[key] || key }
function typeLabel(key) { return TYPE_LABELS[key] || key }
function priorityLabel(key) { return PRIORITY_LABELS[key] || key }

function barWidth(count) {
  const max = Math.max(...(stats.byStatus || []).map((i) => i.count), ...(stats.byType || []).map((i) => i.count), ...(stats.byPriority || []).map((i) => i.count), 1)
  return `${Math.max(2, (count / max) * 100)}%`
}

function trendHeight(count) {
  const max = Math.max(...(stats.monthlyTrend || []).map((i) => i.count), 1)
  return `${Math.max(2, (count / max) * 100)}%`
}

async function load() {
  const data = await ticketsApi.stats()
  Object.assign(stats, data)
}

async function exportCsv() {
  const res = await ticketsApi.statsExport()
  const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `ticket-stats-${new Date().toISOString().slice(0, 10)}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.stat-card {
  padding: 20px;
  border-radius: 12px;
  background: linear-gradient(135deg, var(--el-color-primary-light-8), var(--el-color-primary-light-9));
  border: 1px solid var(--el-border-color-lighter);
}

.stat-value {
  font-size: 30px;
  font-weight: 700;
  color: var(--el-color-primary);
}

.stat-label {
  margin-top: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.chart-row {
  margin-bottom: 16px;
}

.chart-card {
  padding: 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
}

.chart-card h4 {
  margin: 0 0 14px;
}

.bar-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.bar-label {
  width: 96px;
  flex-shrink: 0;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.bar-track {
  flex: 1;
  height: 14px;
  background: var(--el-fill-color-light);
  border-radius: 7px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: 7px;
  background: linear-gradient(90deg, var(--el-color-primary), var(--el-color-primary-light-5));
  transition: width 0.4s ease;
}

.bar-fill.type { background: linear-gradient(90deg, var(--el-color-success), var(--el-color-success-light-5)); }
.bar-fill.priority { background: linear-gradient(90deg, var(--el-color-warning), var(--el-color-warning-light-5)); }

.bar-count {
  width: 40px;
  text-align: right;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.trend {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 160px;
  padding-top: 10px;
}

.trend-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  height: 100%;
}

.trend-value {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}

.trend-bar {
  width: 60%;
  max-width: 32px;
  border-radius: 4px 4px 0 0;
  background: linear-gradient(180deg, var(--el-color-primary), var(--el-color-primary-light-6));
  transition: height 0.4s ease;
}

.trend-label {
  margin-top: 6px;
  font-size: 11px;
  color: var(--el-text-color-secondary);
}
</style>
