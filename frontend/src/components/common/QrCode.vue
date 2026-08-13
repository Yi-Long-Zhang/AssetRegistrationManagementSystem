<template>
  <div class="qr-wrap">
    <canvas ref="canvasRef" class="qr-canvas" />
    <el-button v-if="downloadable" size="small" @click="download">下载 PNG</el-button>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'

const props = defineProps({
  value: { type: String, required: true },
  size: { type: Number, default: 160 },
  downloadable: { type: Boolean, default: false }
})

const canvasRef = ref(null)

async function render() {
  if (!canvasRef.value || !props.value) return
  try {
    await QRCode.toCanvas(canvasRef.value, props.value, { width: props.size, margin: 1 })
  } catch (error) {
    console.error('二维码生成失败', error)
  }
}

function download() {
  const url = canvasRef.value?.toDataURL('image/png')
  if (!url) return
  const link = document.createElement('a')
  link.href = url
  link.download = 'asset-qrcode.png'
  link.click()
}

onMounted(render)
watch(() => props.value, render)
</script>

<style scoped>
.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.qr-canvas {
  image-rendering: pixelated;
}
</style>
