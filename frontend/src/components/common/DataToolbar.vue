<template>
  <div class="data-toolbar">
    <div class="data-toolbar-main">
      <slot />
    </div>
    <div v-if="$slots.actions" class="toolbar-actions">
      <slot name="actions" />
    </div>
  </div>
</template>
