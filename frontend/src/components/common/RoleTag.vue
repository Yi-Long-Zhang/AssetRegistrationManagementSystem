<template>
  <StatusTag :value="value" :map="ROLE_MAP" />
</template>

<script setup>
import StatusTag from './StatusTag.vue'
import { ROLE_MAP } from '../../constants/dictionaries'

defineProps({
  value: { type: String, default: '' }
})
</script>
