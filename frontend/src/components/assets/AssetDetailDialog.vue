<template>
  <el-dialog
    :model-value="modelValue"
    :width="'760px'"
    :show-close="false"
    class="asset-detail-dialog"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <template v-if="asset">
      <!-- Hero 渐变头卡 -->
      <div class="hero">
        <div class="hero-glow"></div>
        <div class="hero-main">
          <div class="hero-title-row">
            <h2 class="hero-title">{{ asset.hostname || '未命名主机' }}</h2>
            <span class="status-badge" :class="onlineClass">
              <span class="status-dot"></span>{{ onlineLabel }}
            </span>
          </div>
          <div class="hero-tags">
            <span class="ip-badge">{{ asset.ip }}</span>
            <el-tag size="small" effect="dark" class="hero-tag">{{ asset.macAddress ? 'MAC ' + asset.macAddress : '无 MAC 信息' }}</el-tag>
            <el-tag size="small" effect="plain" class="hero-tag">{{ asset.assetType || 'server' }}</el-tag>
            <el-tag v-if="asset.sequenceNo" size="small" effect="plain" class="hero-tag">序号 {{ asset.sequenceNo }}</el-tag>
            <el-tag v-if="asset.additionalIPs" size="small" effect="plain" class="hero-tag">关联 {{ asset.additionalIPs }}</el-tag>
          </div>
        </div>
        <div class="hero-icon">
          <el-icon><component :is="assetIcon" /></el-icon>
        </div>
      </div>

      <!-- 指标卡片行 -->
      <div class="metric-row">
        <div class="metric-card" v-for="(m, i) in metrics" :key="m.label" :style="{ animationDelay: (0.1 + i * 0.08) + 's' }">
          <div class="metric-icon" :style="{ background: m.color + '1a', color: m.color }">
            <el-icon><component :is="m.icon" /></el-icon>
          </div>
          <div class="metric-body">
            <div class="metric-value" :ref="m.ref">{{ m.value }}</div>
            <div class="metric-label">{{ m.label }}</div>
          </div>
        </div>
      </div>

      <div class="content-row">
        <!-- 左栏：端口与服务 -->
        <div class="col col-ports">
          <h3 class="col-title">开放端口与服务</h3>
          <div class="port-cloud">
            <el-tooltip v-for="p in portItems" :key="p.port" :content="p.tooltip" placement="top">
              <span class="port-chip" :style="chipStyle(p)">{{ p.port }}<em>{{ p.protocol }}</em></span>
            </el-tooltip>
            <span v-if="!portItems.length" class="empty-text">无开放端口</span>
          </div>
          <div class="port-progress">
            <span class="progress-label">服务识别覆盖</span>
            <el-progress :percentage="serviceCoverage" :stroke-width="8" :show-text="false" color="var(--brand-gradient)" />
          </div>
          <div class="detail-lines">
            <template v-if="asset.runningServices">
              <div v-for="line in serviceLines" :key="line" class="detail-line">{{ line }}</div>
            </template>
            <span v-else class="empty-text">无服务信息</span>
          </div>
        </div>

        <!-- 右栏：变更历史 -->
        <div class="col col-history">
          <h3 class="col-title">变更历史</h3>
          <el-timeline v-loading="loading" class="history-timeline">
            <el-timeline-item
              v-for="snap in history"
              :key="snap.id"
              :timestamp="formatTime(snap.createdAt)"
              placement="top"
              :color="snapColor(snap.changeType)"
              class="timeline-item"
            >
              <div class="snap-head">
                <el-tag size="small" :type="dictItem(snapshotSourceMap, snap.source).type">{{ dictItem(snapshotSourceMap, snap.source).label }}</el-tag>
                <span class="change-chip" :style="changeChipStyle(snap.changeType)">{{ changeLabel(snap.changeType) }}</span>
              </div>
              <pre v-if="snap.diffSummary" class="snap-diff">{{ snap.diffSummary }}</pre>
              <span v-else class="snap-empty">无字段变化（创建或首次快照）</span>
            </el-timeline-item>
            <el-empty v-if="!loading && !history.length" description="暂无变更历史" :image-size="60" />
          </el-timeline>
        </div>
      </div>

      <div class="footer-note">
        <el-icon><InfoFilled /></el-icon>
        <span>资产编号 {{ asset.assetNo }} · 最近发现 {{ formatTime(asset.lastSeenAt) }} · 首次发现 {{ formatTime(asset.discoveredAt) }}</span>
      </div>
    </template>
    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">关 闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import {
  Clock,
  Cpu,
  Connection,
  InfoFilled,
  Monitor,
  Service
} from '@element-plus/icons-vue'
import { discoveryApi } from '../../api/discovery'
import { ONLINE_STATUS_MAP, SNAPSHOT_SOURCE_MAP, dictItem } from '../../constants/dictionaries'

const props = defineProps({
  modelValue: Boolean,
  asset: { type: Object, default: null }
})
const emit = defineEmits(['update:modelValue'])

const history = ref([])
const loading = ref(false)

const snapshotSourceMap = SNAPSHOT_SOURCE_MAP
const onlineMap = ONLINE_STATUS_MAP

const assetIcon = computed(() => Monitor)
const onlineClass = computed(() => (props.asset?.onlineStatus || 'unknown'))
const onlineLabel = computed(() => dictItem(onlineMap, props.asset?.onlineStatus).label)

const metrics = computed(() => {
  const asset = props.asset || {}
  return [
    { label: 'IP 地址', value: asset.ip || '-', icon: Connection, color: '#6366f1', ref: 'm0' },
    { label: '操作系统', value: asset.os || '-', icon: Cpu, color: '#8b5cf6', ref: 'm1' },
    { label: '开放端口', value: String(portItems.value.length || 0), icon: Service, color: '#0ea5e9', ref: 'm2', animate: true },
    { label: '资产类型', value: asset.assetType || '-', icon: Monitor, color: '#10b981', ref: 'm3' }
  ]
})

const portItems = computed(() => {
  const raw = (props.asset?.openPorts || '').split(',').map((s) => s.trim()).filter(Boolean)
  return raw.map((item) => {
    const [port, protocol = 'tcp'] = item.split('/')
    return { port, protocol, tooltip: serviceForPort(port) || item }
  })
})

function serviceForPort(port) {
  const services = (props.asset?.runningServices || '').split('\n').map((s) => s.trim()).filter(Boolean)
  for (const line of services) {
    if (line.startsWith(port + '/')) {
      return line.split(':').slice(1).join(':').trim() || line
    }
  }
  return ''
}

const serviceLines = computed(() =>
  (props.asset?.runningServices || '').split('\n').map((s) => s.trim()).filter(Boolean)
)

const serviceCoverage = computed(() => {
  if (!portItems.value.length) return 0
  const withService = portItems.value.filter((p) => p.tooltip && p.tooltip !== p.port + '/tcp').length
  return Math.round((withService / portItems.value.length) * 100)
})

function chipStyle(p) {
  const color = portColor(p.tooltip)
  return { background: color + '1a', color, border: '1px solid ' + color + '40' }
}

function portColor(tooltip) {
  const t = tooltip.toLowerCase()
  if (/(http|https|web|nginx|apache)/.test(t)) return '#10b981'
  if (/(msrpc|rpc)/.test(t)) return '#6366f1'
  if (/(microsoft-ds|smb|netbios)/.test(t)) return '#f59e0b'
  if (/(ssh|secure)/.test(t)) return '#8b5cf6'
  if (/(sql|postgres|mysql|oracle|redis|mongo|database)/.test(t)) return '#0ea5e9'
  if (/(ftp|telnet|snmp)/.test(t)) return '#ef4444'
  return '#6b7280'
}

const changeMeta = {
  create: { label: '创建', color: '#10b981' },
  update: { label: '更新', color: '#f59e0b' },
  offline: { label: '离线', color: '#ef4444' },
  online: { label: '恢复在线', color: '#6366f1' },
  none: { label: '无变化', color: '#9ca3af' }
}

function snapColor(changeType) {
  return changeMeta[changeType]?.color || '#9ca3af'
}

function changeChipStyle(changeType) {
  const c = changeMeta[changeType] || changeMeta.none
  return { background: c.color + '1a', color: c.color, border: '1px solid ' + c.color + '40' }
}

function changeLabel(changeType) {
  return changeMeta[changeType]?.label || changeType
}

function formatTime(value) {
  if (!value) return '-'
  const d = new Date(value)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function loadHistory() {
  if (!props.asset) return
  history.value = []
  loading.value = true
  try {
    history.value = await discoveryApi.assetHistory(props.asset.id)
  } catch {
    history.value = []
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.modelValue, props.asset?.id],
  ([visible, id]) => {
    if (visible && id) loadHistory()
  },
  { immediate: true }
)
</script>

<style scoped>
.asset-detail-dialog {
  --el-dialog-border-radius: 24px;
}

.asset-detail-dialog :deep(.el-dialog) {
  overflow: hidden;
}

.asset-detail-dialog :deep(.el-dialog__body) {
  padding: 0 24px 8px;
}

/* ---------- Hero ---------- */
.hero {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 26px 26px 22px;
  margin: 0 -24px 20px;
  border-radius: var(--radius-xl) var(--radius-xl) 0 0;
  overflow: hidden; /* 裁切光晕与背景超出部分，与弹窗圆角衔接 */
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 55%, #a855f7 100%);
  animation: hero-in 0.5s ease both;
}

@keyframes hero-in {
  from {
    opacity: 0;
    transform: translateY(-12px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.hero-glow {
  position: absolute;
  right: -40px;
  top: -60px;
  width: 220px;
  height: 220px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.14);
  filter: blur(30px);
}

.hero-main {
  position: relative;
  z-index: 1;
  min-width: 0;
}

.hero-title-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.hero-title {
  margin: 0;
  color: #fff;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

/* 在线状态呼吸灯徽章 */
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(4px);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #9ca3af;
  animation: pulse 1.6s ease-in-out infinite;
}

.status-badge.online .status-dot {
  background: #34d399;
  box-shadow: 0 0 8px rgba(52, 211, 153, 0.9);
}

.status-badge.offline .status-dot {
  background: #f87171;
  box-shadow: 0 0 8px rgba(248, 113, 113, 0.9);
}

.status-badge.unknown .status-dot {
  background: #fbbf24;
  box-shadow: 0 0 8px rgba(251, 191, 36, 0.8);
}

@keyframes pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.4);
    opacity: 0.65;
  }
}

.hero-tags {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  flex-wrap: wrap;
}

.ip-badge {
  padding: 2px 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  font-family: 'JetBrains Mono', Consolas, monospace;
}

.hero-tag {
  border-radius: 999px;
}

.hero-icon {
  position: relative;
  z-index: 1;
  width: 72px;
  height: 72px;
  display: grid;
  place-items: center;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.14);
  color: #fff;
  font-size: 40px;
  flex-shrink: 0;
}

/* ---------- 指标卡 ---------- */
.metric-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 18px;
}

.metric-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  background: #fff;
  border: 1px solid var(--el-border-color-light);
  border-radius: 14px;
  box-shadow: var(--shadow-card);
  opacity: 0;
  animation: rise 0.45s ease forwards;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.metric-card:hover {
  transform: translateY(-3px);
  box-shadow: var(--shadow-hover);
}

@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.metric-icon {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border-radius: 12px;
  font-size: 20px;
  flex-shrink: 0;
}

.metric-body {
  min-width: 0;
}

.metric-value {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-label {
  font-size: 12px;
  color: var(--text-secondary);
}

/* ---------- 内容双栏 ---------- */
.content-row {
  display: grid;
  grid-template-columns: 1.15fr 1fr;
  gap: 18px;
}

.col {
  min-width: 0;
}

.col-title {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 8px;
}

.col-title::before {
  content: '';
  width: 4px;
  height: 16px;
  border-radius: 2px;
  background: var(--brand-gradient);
}

.port-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

.port-chip {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 700;
  font-family: 'JetBrains Mono', Consolas, monospace;
  cursor: default;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
  animation: rise 0.4s ease backwards;
}

.port-chip:hover {
  transform: scale(1.08) translateY(-2px);
  box-shadow: 0 4px 12px rgba(17, 24, 39, 0.12);
}

.port-chip em {
  font-style: normal;
  font-size: 10px;
  font-weight: 500;
  opacity: 0.65;
}

.port-progress {
  margin-bottom: 12px;
}

.progress-label {
  display: block;
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 4px;
}

.detail-lines {
  max-height: 150px;
  overflow-y: auto;
}

.detail-line {
  padding: 5px 10px;
  margin-bottom: 4px;
  background: #f8f9fc;
  border-radius: 8px;
  font-size: 12px;
  color: #374151;
  font-family: 'JetBrains Mono', Consolas, monospace;
  word-break: break-all;
}

/* ---------- 变更历史 ---------- */
.history-timeline {
  padding-left: 4px;
  max-height: 300px;
  overflow-y: auto;
}

.timeline-item :deep(.el-timeline-item__node) {
  box-shadow: 0 0 0 4px rgba(99, 102, 241, 0.12);
  animation: pulse-ring 2s ease-in-out infinite;
}

@keyframes pulse-ring {
  0%,
  100% {
    box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1);
  }
  50% {
    box-shadow: 0 0 0 7px rgba(99, 102, 241, 0.04);
  }
}

.timeline-item :deep(.el-timeline-item__content) {
  background: #fff;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  padding: 8px 12px;
  box-shadow: var(--shadow-card);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  animation: rise 0.4s ease backwards;
}

.timeline-item :deep(.el-timeline-item__content:hover) {
  transform: translateY(-2px);
  box-shadow: var(--shadow-hover);
}

.snap-head {
  display: flex;
  gap: 8px;
  margin-bottom: 6px;
  align-items: center;
}

.change-chip {
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

.snap-diff {
  margin: 0;
  font-size: 12px;
  color: #374151;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'JetBrains Mono', Consolas, monospace;
  line-height: 1.6;
}

.snap-empty {
  color: #9ca3af;
  font-size: 12px;
}

.empty-text {
  color: #9ca3af;
  font-size: 13px;
}

.footer-note {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px dashed var(--el-border-color-light);
  color: var(--text-secondary);
  font-size: 12px;
}

@media (max-width: 720px) {
  .metric-row {
    grid-template-columns: repeat(2, 1fr);
  }
  .content-row {
    grid-template-columns: 1fr;
  }
}
</style>
