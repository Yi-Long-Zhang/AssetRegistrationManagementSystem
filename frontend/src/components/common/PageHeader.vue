<template>
  <div class="page-header">
    <div>
      <h2>{{ title }}</h2>
      <p v-if="description" class="muted">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="page-header-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' }
})
</script>
