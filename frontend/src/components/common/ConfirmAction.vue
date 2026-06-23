<template>
  <el-button v-bind="$attrs" @click="confirm">
    <slot />
  </el-button>
</template>

<script setup>
import { ElMessageBox } from 'element-plus'

const props = defineProps({
  title: { type: String, default: '操作确认' },
  message: { type: String, required: true },
  type: { type: String, default: 'warning' }
})

const emit = defineEmits(['confirm'])

async function confirm() {
  try {
    await ElMessageBox.confirm(props.message, props.title, { type: props.type })
    emit('confirm')
  } catch {
    // User canceled the confirmation dialog.
  }
}
</script>
