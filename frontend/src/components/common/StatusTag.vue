<template>
  <el-tag :type="item.type" effect="light" round>{{ item.label }}</el-tag>
</template>

<script setup>
import { computed } from 'vue'
import { dictItem } from '../../constants/dictionaries'

const props = defineProps({
  value: { type: String, default: '' },
  map: { type: Object, required: true }
})

const item = computed(() => dictItem(props.map, props.value))
</script>
