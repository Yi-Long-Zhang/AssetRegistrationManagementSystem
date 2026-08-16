<template>
  <div class="print-page">
    <div class="no-print toolbar">
      <el-button type="primary" @click="window.print()">打印</el-button>
      <el-button @click="window.close()">关闭</el-button>
      <span class="muted">{{ assets.length }} 个标签</span>
    </div>
    <div class="labels">
      <div v-for="asset in assets" :key="asset.id" class="label">
        <div class="label-qr"><QrCode :value="labelUrl(asset)" :size="90" /></div>
        <div class="label-info">
          <div class="label-no">{{ asset.assetNo }}</div>
          <div class="label-host">{{ asset.hostname }}</div>
          <div class="label-ip">{{ asset.ip }}</div>
          <div class="label-type">{{ asset.assetType || '-' }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { assetsApi } from '../api'
import QrCode from '../components/common/QrCode.vue'

const route = useRoute()
const assets = ref([])

function labelUrl(asset) {
  return `${window.location.origin}/assets?assetId=${asset.id}`
}

onMounted(async () => {
  const ids = String(route.query.ids || '').split(',').filter(Boolean)
  if (!ids.length) return
  // 后端 pageSize 上限 200：分页拉全量（或已覆盖全部选中 id）后按 id 匹配，避免超量静默丢标签
  const map = new Map()
  for (let page = 1; ; page++) {
    const data = await assetsApi.list({ page, pageSize: 200 })
    for (const a of data.items || []) map.set(String(a.id), a)
    if (!data.items?.length || map.size >= (data.total || 0) || ids.every((id) => map.has(id))) break
  }
  assets.value = ids.map((id) => map.get(id)).filter(Boolean)
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
  color: #999;
  font-size: 13px;
}

.labels {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.label {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 320px;
  padding: 12px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
}

.label-info {
  font-size: 12px;
  line-height: 1.5;
}

.label-no {
  font-weight: 700;
  font-size: 14px;
}

.label-host {
  color: #333;
}

.label-ip,
.label-type {
  color: #666;
}

@media print {
  .no-print {
    display: none;
  }

  .label {
    border: 1px dashed #999;
    break-inside: avoid;
  }
}
</style>
